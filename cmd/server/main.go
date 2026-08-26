package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Aditya7880900936/parcelpilot-ai/internal/action"
	"github.com/Aditya7880900936/parcelpilot-ai/internal/agent"
	"github.com/Aditya7880900936/parcelpilot-ai/internal/db"
	"github.com/Aditya7880900936/parcelpilot-ai/internal/embeddings"
	"github.com/Aditya7880900936/parcelpilot-ai/internal/retrieval"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

const confirmationTTL = 10 * time.Minute

type evaluateRequest struct {
	Query     string `json:"query"`
	AccountID string `json:"account_id"`
	OrderID   string `json:"order_id"`
}

type evaluateResponse struct {
	Answer               string         `json:"answer"`
	Confidence           float64        `json:"confidence"`
	Sources              []agent.Source `json:"sources,omitempty"`
	Action               *agent.Action  `json:"action,omitempty"`
	Escalate             bool           `json:"escalate"`
	Reason               string         `json:"reason"`
	ConfirmationRequired bool           `json:"confirmation_required"`
	ConfirmationID       string         `json:"confirmation_id,omitempty"`
}

type confirmRequest struct {
	ConfirmationID string `json:"confirmation_id"`
	Confirm        bool   `json:"confirm"`
}

type confirmResponse struct {
	Status  string        `json:"status"`
	Message string        `json:"message"`
	Action  *agent.Action `json:"action,omitempty"`
}

type pendingConfirmation struct {
	Action    *agent.Action
	ExpiresAt time.Time
}

type confirmationStore struct {
	mu      sync.Mutex
	items   map[string]pendingConfirmation
	counter uint64
}

func newConfirmationStore() *confirmationStore {
	return &confirmationStore{
		items: make(map[string]pendingConfirmation),
	}
}

func (s *confirmationStore) create(action *agent.Action) (string, error) {
	if action == nil {
		return "", fmt.Errorf("action is nil")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.counter++

	id := fmt.Sprintf(
		"confirm-%d-%d",
		time.Now().UnixNano(),
		s.counter,
	)

	s.items[id] = pendingConfirmation{
		Action: &agent.Action{
			Type:   action.Type,
			Target: action.Target,
			Reason: action.Reason,
		},
		ExpiresAt: time.Now().Add(confirmationTTL),
	}

	return id, nil
}

func (s *confirmationStore) take(id string) (*agent.Action, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	pending, ok := s.items[id]
	if !ok {
		return nil, false
	}

	delete(s.items, id)

	if time.Now().After(pending.ExpiresAt) {
		return nil, false
	}

	return pending.Action, true
}

func (s *confirmationStore) reject(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	pending, ok := s.items[id]
	if !ok {
		return false
	}

	delete(s.items, id)

	if time.Now().After(pending.ExpiresAt) {
		return false
	}

	return true
}

func (s *confirmationStore) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	for id, pending := range s.items {
		if now.After(pending.ExpiresAt) {
			delete(s.items, id)
		}
	}
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Printf("warning: .env not loaded: %v", err)
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		log.Fatalf("create database pool: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("database ping failed: %v", err)
	}

	provider := embeddings.NewOllamaProvider(
		getEnv("OLLAMA_BASE_URL", "http://localhost:11434"),
		getEnv("OLLAMA_EMBEDDING_MODEL", "nomic-embed-text"),
	)

	retriever := retrieval.NewPostgresRetriever(pool)

	retrievalService := retrieval.NewService(
		provider,
		retriever,
	)

	agentRetriever := agent.NewRetrieverAdapter(retrievalService)

	accountRepository := db.NewAccountContextRepository(pool)
	contextLoader := db.NewContextLoader(accountRepository)

	decisionEngine := agent.NewDecisionEngine()
	contextBuilder := agent.NewContextBuilder()

	orchestrator := agent.NewOrchestrator(
		agentRetriever,
		contextLoader,
		decisionEngine,
		contextBuilder,
	)

	executor := action.NewExecutor(pool)

	confirmations := newConfirmationStore()

	// Remove expired confirmations periodically.
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			confirmations.cleanup()
		}
	}()

	mux := http.NewServeMux()

	// ------------------------------------------------------------
	// Chat UI
	// ------------------------------------------------------------

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(chatHTML))
	})

	// ------------------------------------------------------------
	// Health
	// ------------------------------------------------------------

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
				"error": "method not allowed",
			})
			return
		}

		healthCtx, cancel := context.WithTimeout(
			r.Context(),
			2*time.Second,
		)
		defer cancel()

		if err := pool.Ping(healthCtx); err != nil {
			log.Printf("health check failed: %v", err)

			writeJSON(w, http.StatusServiceUnavailable, map[string]string{
				"status": "unhealthy",
			})
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{
			"status": "ok",
		})
	})

	// ------------------------------------------------------------
	// Evaluate
	//
	// IMPORTANT:
	// This endpoint NEVER executes a state-changing action.
	// It only evaluates the request and, when appropriate,
	// creates a pending confirmation.
	// ------------------------------------------------------------

	mux.HandleFunc("/evaluate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
				"error": "method not allowed",
			})
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		defer r.Body.Close()

		var req evaluateRequest

		decoder := json.NewDecoder(r.Body)

		if err := decoder.Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "invalid JSON body",
			})
			return
		}

		// Reject multiple JSON values in the same request.
		var extra any
		if err := decoder.Decode(&extra); err != io.EOF {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "request body must contain a single JSON object",
			})
			return
		}

		req.Query = strings.TrimSpace(req.Query)
		req.AccountID = strings.TrimSpace(req.AccountID)
		req.OrderID = strings.TrimSpace(req.OrderID)

		if req.Query == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "query is required",
			})
			return
		}

		if req.AccountID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "account_id is required",
			})
			return
		}

		if req.OrderID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "order_id is required",
			})
			return
		}

		requestCtx, cancel := context.WithTimeout(
			r.Context(),
			2*time.Minute,
		)
		defer cancel()

		response, err := orchestrator.Evaluate(
			requestCtx,
			req.Query,
			req.AccountID,
			req.OrderID,
		)
		if err != nil {
			log.Printf("agent evaluation failed: %v", err)

			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "agent evaluation failed",
			})
			return
		}

		result := evaluateResponse{
			Answer:     response.Answer,
			Confidence: response.Confidence,
			Sources:    response.Sources,
			Action:     response.Action,
			Escalate:   response.Escalate,
			Reason:     response.Reason,
		}

		// --------------------------------------------------------
		// Confirmation gate
		// --------------------------------------------------------
		//
		// No state-changing action is executed here.
		//
		// High-confidence deterministic action:
		//     create pending confirmation
		//
		// Low-confidence action:
		//     return proposal only
		// --------------------------------------------------------

		if response.Action != nil &&
			response.Confidence >= 0.90 {

			confirmationID, err := confirmations.create(
				response.Action,
			)
			if err != nil {
				log.Printf(
					"create confirmation failed: %v",
					err,
				)

				writeJSON(w, http.StatusInternalServerError, map[string]string{
					"error": "failed to create confirmation",
				})
				return
			}

			result.ConfirmationRequired = true
			result.ConfirmationID = confirmationID

			log.Printf(
				"action awaiting confirmation: type=%s target=%s confidence=%.4f confirmation_id=%s",
				response.Action.Type,
				response.Action.Target,
				response.Confidence,
				confirmationID,
			)
		}

		writeJSON(w, http.StatusOK, result)
	})

	// ------------------------------------------------------------
	// Confirm / Reject
	// ------------------------------------------------------------

	mux.HandleFunc("/confirm", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
				"error": "method not allowed",
			})
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
		defer r.Body.Close()

		var req confirmRequest

		decoder := json.NewDecoder(r.Body)

		if err := decoder.Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "invalid JSON body",
			})
			return
		}

		var extra any
		if err := decoder.Decode(&extra); err != io.EOF {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "request body must contain a single JSON object",
			})
			return
		}

		req.ConfirmationID = strings.TrimSpace(
			req.ConfirmationID,
		)

		if req.ConfirmationID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "confirmation_id is required",
			})
			return
		}

		// --------------------------------------------------------
		// Explicit rejection
		// --------------------------------------------------------

		if !req.Confirm {
			if confirmations.reject(req.ConfirmationID) {
				log.Printf(
					"action rejected: confirmation_id=%s",
					req.ConfirmationID,
				)

				writeJSON(w, http.StatusOK, confirmResponse{
					Status:  "rejected",
					Message: "action rejected; no state was changed",
				})
				return
			}

			writeJSON(w, http.StatusNotFound, map[string]string{
				"error": "confirmation not found or expired",
			})
			return
		}

		// --------------------------------------------------------
		// Explicit approval
		// --------------------------------------------------------

		pendingAction, ok := confirmations.take(
			req.ConfirmationID,
		)

		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{
				"error": "confirmation not found or expired",
			})
			return
		}

		requestCtx, cancel := context.WithTimeout(
			r.Context(),
			30*time.Second,
		)
		defer cancel()

		if err := executor.Execute(
			requestCtx,
			pendingAction,
		); err != nil {
			log.Printf(
				"confirmed action execution failed: type=%s target=%s error=%v",
				pendingAction.Type,
				pendingAction.Target,
				err,
			)

			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "action execution failed",
			})
			return
		}

		log.Printf(
			"confirmed action executed: type=%s target=%s confirmation_id=%s",
			pendingAction.Type,
			pendingAction.Target,
			req.ConfirmationID,
		)

		writeJSON(w, http.StatusOK, confirmResponse{
			Status:  "executed",
			Message: "action confirmed and executed successfully",
			Action:  pendingAction,
		})
	})

	addr := getEnv("SERVER_ADDR", ":8080")

	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      2 * time.Minute,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("ParcelPilot AI server listening on %s", addr)

	if err := server.ListenAndServe(); err != nil &&
		err != http.ErrServerClosed {
		log.Fatalf("server failed: %v", err)
	}
}

func writeJSON(
	w http.ResponseWriter,
	status int,
	value any,
) {
	w.Header().Set(
		"Content-Type",
		"application/json",
	)

	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("failed to write JSON response: %v", err)
	}
}

func getEnv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))

	if value == "" {
		return fallback
	}

	return value
}

const chatHTML = `<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>ParcelPilot AI</title>

	<style>
		* {
			box-sizing: border-box;
		}

		body {
			margin: 0;
			font-family:
				Inter,
				system-ui,
				-apple-system,
				BlinkMacSystemFont,
				"Segoe UI",
				sans-serif;
			background: #f8fafc;
			color: #0f172a;
		}

		.container {
			width: min(900px, calc(100% - 32px));
			margin: 40px auto;
		}

		.header {
			margin-bottom: 20px;
		}

		.header h1 {
			margin: 0;
			font-size: 28px;
		}

		.header p {
			margin-top: 6px;
			color: #64748b;
		}

		.chat {
			background: white;
			border: 1px solid #e2e8f0;
			border-radius: 16px;
			padding: 20px;
			box-shadow: 0 10px 30px rgba(15, 23, 42, 0.06);
		}

		.messages {
			min-height: 320px;
			max-height: 560px;
			overflow-y: auto;
			margin-bottom: 20px;
		}

		.message {
			margin-bottom: 14px;
			padding: 14px;
			border-radius: 12px;
			white-space: pre-wrap;
		}

		.message.user {
			background: #eff6ff;
			margin-left: 80px;
		}

		.message.agent {
			background: #f8fafc;
			border: 1px solid #e2e8f0;
			margin-right: 80px;
		}

		.meta {
			margin-top: 8px;
			font-size: 12px;
			color: #64748b;
		}

		.confirmation {
			margin-top: 12px;
			padding: 14px;
			border: 1px solid #f59e0b;
			background: #fffbeb;
			border-radius: 10px;
		}

		.confirmation-title {
			font-weight: 700;
			margin-bottom: 8px;
		}

		.confirmation-details {
			font-size: 14px;
			white-space: pre-wrap;
			margin-bottom: 12px;
		}

		button {
			border: 0;
			border-radius: 8px;
			padding: 9px 14px;
			cursor: pointer;
			font-weight: 600;
		}

		button:disabled {
			opacity: 0.5;
			cursor: not-allowed;
		}

		.confirm {
			background: #16a34a;
			color: white;
			margin-right: 8px;
		}

		.reject {
			background: #dc2626;
			color: white;
		}

		form {
			display: grid;
			gap: 10px;
		}

		input,
		textarea {
			width: 100%;
			padding: 11px 12px;
			border: 1px solid #cbd5e1;
			border-radius: 8px;
			font: inherit;
		}

		textarea {
			min-height: 90px;
			resize: vertical;
		}

		.submit {
			background: #0f172a;
			color: white;
		}

		.status {
			font-size: 13px;
			color: #64748b;
			min-height: 20px;
		}
	</style>
</head>

<body>
	<div class="container">
		<div class="header">
			<h1>ParcelPilot AI</h1>
			<p>Policy-aware logistics support agent</p>
		</div>

		<div class="chat">
			<div id="messages" class="messages"></div>

			<form id="chat-form">
				<input
					id="account"
					placeholder="Account ID e.g. ACCT-001"
					value="ACCT-001"
					required
				/>

				<input
					id="order"
					placeholder="Order ID e.g. ORD-1001"
					value="ORD-1001"
					required
				/>

				<textarea
					id="query"
					placeholder="Ask about the shipment..."
					required
				></textarea>

				<button class="submit" type="submit">
					Ask ParcelPilot
				</button>

				<div id="status" class="status"></div>
			</form>
		</div>
	</div>

<script>
const form = document.getElementById("chat-form");
const queryInput = document.getElementById("query");
const accountInput = document.getElementById("account");
const orderInput = document.getElementById("order");
const messages = document.getElementById("messages");
const statusText = document.getElementById("status");

function addMessage(text, type) {
	const message = document.createElement("div");
	message.className = "message " + type;
	message.textContent = text;

	messages.appendChild(message);
	messages.scrollTop = messages.scrollHeight;

	return message;
}

function addActionCard(parent, data) {
	const card = document.createElement("div");
	card.className = "confirmation";

	const title = document.createElement("div");
	title.className = "confirmation-title";
	title.textContent = "⚠️ Confirmation required";

	const details = document.createElement("div");
	details.className = "confirmation-details";

	details.textContent =
		data.action.Type +
		" → " +
		data.action.Target +
		"\nReason: " +
		data.action.Reason;

	const confirmButton = document.createElement("button");
	confirmButton.className = "confirm";
	confirmButton.textContent = "Confirm action";

	const rejectButton = document.createElement("button");
	rejectButton.className = "reject";
	rejectButton.textContent = "Reject";

	confirmButton.onclick = async () => {
		confirmButton.disabled = true;
		rejectButton.disabled = true;

		await confirmAction(
			data.confirmation_id,
			true,
			card
		);
	};

	rejectButton.onclick = async () => {
		confirmButton.disabled = true;
		rejectButton.disabled = true;

		await confirmAction(
			data.confirmation_id,
			false,
			card
		);
	};

	card.appendChild(title);
	card.appendChild(details);
	card.appendChild(confirmButton);
	card.appendChild(rejectButton);

	parent.appendChild(card);

	messages.scrollTop = messages.scrollHeight;
}

async function confirmAction(id, confirm, card) {
	try {
		const response = await fetch("/confirm", {
			method: "POST",
			headers: {
				"Content-Type": "application/json"
			},
			body: JSON.stringify({
				confirmation_id: id,
				confirm: confirm
			})
		});

		const data = await response.json();

		if (!response.ok) {
			throw new Error(
				data.error || "confirmation failed"
			);
		}

		card.innerHTML = "";

		const result = document.createElement("strong");

		result.textContent =
			data.status === "executed"
				? "✅ Action executed successfully"
				: "❌ Action rejected";

		card.appendChild(result);

		if (data.message) {
			const message = document.createElement("div");

			message.textContent = data.message;

			card.appendChild(message);
		}
	} catch (error) {
		card.innerHTML = "";

		const message = document.createElement("strong");

		message.textContent =
			"Action failed: " + error.message;

		card.appendChild(message);
	}
}

form.addEventListener("submit", async (event) => {
	event.preventDefault();

	const query = queryInput.value.trim();
	const accountID = accountInput.value.trim();
	const orderID = orderInput.value.trim();

	if (!query || !accountID || !orderID) {
		addMessage(
			"Please provide query, account ID and order ID.",
			"agent"
		);
		return;
	}

	addMessage(query, "user");

	queryInput.value = "";

	statusText.textContent =
		"Agent is evaluating...";

	try {
		const response = await fetch("/evaluate", {
			method: "POST",
			headers: {
				"Content-Type": "application/json"
			},
			body: JSON.stringify({
				query: query,
				account_id: accountID,
				order_id: orderID
			})
		});

		const data = await response.json();

		if (!response.ok) {
			throw new Error(
				data.error || "evaluation failed"
			);
		}

		const message = addMessage(
			data.answer || "No answer returned.",
			"agent"
		);

		const meta = document.createElement("div");
		meta.className = "meta";

		meta.textContent =
			"Confidence: " +
			Number(data.confidence).toFixed(2) +
			(data.escalate
				? " • Escalation recommended"
				: "");

		message.appendChild(meta);

		if (data.confirmation_required) {
			addActionCard(message, data);
		}

		if (
			data.action &&
			!data.confirmation_required
		) {
			const note = document.createElement("div");

			note.className = "meta";

			note.textContent =
				"Action proposed but confirmation was not requested because confidence is below the execution threshold.";

			message.appendChild(note);
		}
	} catch (error) {
		addMessage(
			"Error: " + error.message,
			"agent"
		);
	} finally {
		statusText.textContent = "";
	}
});

addMessage(
	"Hi! Ask me about a shipment. State-changing actions require your explicit confirmation.",
	"agent"
);
</script>

</body>
</html>`
