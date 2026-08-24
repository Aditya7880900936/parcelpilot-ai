package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/Aditya7880900936/parcelpilot-ai/internal/db"
	"github.com/Aditya7880900936/parcelpilot-ai/internal/embeddings"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

const (
	defaultBatchSize         = 8
	defaultEmbeddingProvider = "ollama"
	defaultOllamaURL         = "http://localhost:11434"
	defaultOllamaModel       = "nomic-embed-text"
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

	provider, err := createEmbeddingProvider()
	if err != nil {
		log.Fatalf("create embedding provider: %v", err)
	}

	service := embeddings.NewService(provider)

	repository := db.NewChunkRepository(pool)
	indexer := embeddings.NewIndexer(repository, service)

	batchSize := defaultBatchSize

	if value := os.Getenv("EMBEDDING_BATCH_SIZE"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed <= 0 {
			log.Fatalf("invalid EMBEDDING_BATCH_SIZE: %q", value)
		}

		batchSize = parsed
	}

	log.Printf("Starting embedding indexer (batch=%d)", batchSize)

	if err := indexer.Index(ctx, batchSize); err != nil {
		log.Fatalf("embedding indexing failed: %v", err)
	}

	log.Println("✓ Embedding indexing completed")
}

func createEmbeddingProvider() (embeddings.Provider, error) {
	provider := os.Getenv("EMBEDDING_PROVIDER")

	if provider == "" {
		provider = defaultEmbeddingProvider
	}

	switch provider {
	case "ollama":
		baseURL := os.Getenv("OLLAMA_BASE_URL")
		if baseURL == "" {
			baseURL = defaultOllamaURL
		}

		model := os.Getenv("OLLAMA_EMBEDDING_MODEL")
		if model == "" {
			model = defaultOllamaModel
		}

		log.Printf(
			"Using Ollama embedding provider: model=%s url=%s",
			model,
			baseURL,
		)

		return embeddings.NewOllamaProvider(baseURL, model), nil

	case "openai":
		apiKey := os.Getenv("OPENAI_API_KEY")
		if apiKey == "" {
			return nil, os.ErrNotExist
		}

		log.Println("Using OpenAI embedding provider")

		return embeddings.NewOpenAIProvider(apiKey), nil

	default:
		return nil, os.ErrInvalid
	}
}
