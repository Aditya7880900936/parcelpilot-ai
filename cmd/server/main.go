package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/Aditya7880900936/parcelpilot-ai/internal/action"
	"github.com/Aditya7880900936/parcelpilot-ai/internal/agent"
	"github.com/Aditya7880900936/parcelpilot-ai/internal/db"
	"github.com/Aditya7880900936/parcelpilot-ai/internal/embeddings"
	"github.com/Aditya7880900936/parcelpilot-ai/internal/retrieval"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

type evaluateRequest struct {
	Query     string `json:"query"`
	AccountID string `json:"account_id"`
	OrderID   string `json:"order_id"`
}

type evaluateResponse struct {
	Answer     string         `json:"answer"`
	Confidence float64        `json:"confidence"`
	Sources    []agent.Source `json:"sources"`
	Action     *agent.Action  `json:"action,omitempty"`
	Escalate   bool           `json:"escalate"`
	Reason     string         `json:"reason"`
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

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"status": "ok",
		})
	})

	mux.HandleFunc("/evaluate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
				"error": "method not allowed",
			})
			return
		}

		var req evaluateRequest

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "invalid JSON body",
			})
			return
		}

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

		// Execute only high-confidence deterministic actions.
		if response.Action != nil &&
			!response.Escalate &&
			response.Confidence >= 0.90 {

			if err := executor.Execute(requestCtx, response.Action); err != nil {
				log.Printf("action execution failed: %v", err)

				writeJSON(w, http.StatusInternalServerError, map[string]string{
					"error": "action execution failed",
				})
				return
			}
		}

		writeJSON(w, http.StatusOK, evaluateResponse{
			Answer:     response.Answer,
			Confidence: response.Confidence,
			Sources:    response.Sources,
			Action:     response.Action,
			Escalate:   response.Escalate,
			Reason:     response.Reason,
		})
	})

	addr := getEnv("SERVER_ADDR", ":8080")

	log.Printf("ParcelPilot server listening on %s", addr)

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("write JSON response: %v", err)
	}
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)

	if value == "" {
		return fallback
	}

	return value
}
