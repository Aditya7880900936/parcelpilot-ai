package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/Aditya7880900936/parcelpilot-ai/internal/agent"
	"github.com/Aditya7880900936/parcelpilot-ai/internal/db"
	"github.com/Aditya7880900936/parcelpilot-ai/internal/embeddings"
	"github.com/Aditya7880900936/parcelpilot-ai/internal/retrieval"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Printf("warning: .env not loaded: %v", err)
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		2*time.Minute,
	)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		log.Fatalf("create database pool: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("database ping failed: %v", err)
	}

	// Embedding provider.
	provider := embeddings.NewOllamaProvider(
		getEnv("OLLAMA_BASE_URL", "http://localhost:11434"),
		getEnv("OLLAMA_EMBEDDING_MODEL", "nomic-embed-text"),
	)

	// Retrieval layer.
	retriever := retrieval.NewPostgresRetriever(pool)

	retrievalService := retrieval.NewService(
		provider,
		retriever,
	)

	agentRetriever := agent.NewRetrieverAdapter(retrievalService)

	// Structured data layer.
	accountRepository := db.NewAccountContextRepository(pool)
	contextLoader := db.NewContextLoader(accountRepository)

	// Agent layer.
	decisionEngine := agent.NewDecisionEngine()
	contextBuilder := agent.NewContextBuilder()

	orchestrator := agent.NewOrchestrator(
		agentRetriever,
		contextLoader,
		decisionEngine,
		contextBuilder,
	)

	query := "Can Northstar cancel ORD-1001?"

	log.Printf("Query: %s", query)

	response, err := orchestrator.Evaluate(
		ctx,
		query,
		"ACCT-001",
		"ORD-1001",
	)
	if err != nil {
		log.Fatalf("agent evaluation failed: %v", err)
	}

	log.Println("\n========== AGENT RESPONSE ==========")
	log.Printf("Answer: %s", response.Answer)
	log.Printf("Reason: %s", response.Reason)
	log.Printf("Confidence: %.4f", response.Confidence)
	log.Printf("Escalate: %t", response.Escalate)

	if response.Action != nil {
		log.Printf(
			"Action: %s → %s (%s)",
			response.Action.Type,
			response.Action.Target,
			response.Action.Reason,
		)
	}

	log.Println("\n========== SOURCES ==========")

	for i, source := range response.Sources {
		log.Printf(
			"\n--- Source %d ---\nDocument ID: %d\nScore: %.4f\n%s",
			i+1,
			source.DocumentID,
			source.Score,
			source.Content,
		)
	}
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)

	if value == "" {
		return fallback
	}

	return value
}
