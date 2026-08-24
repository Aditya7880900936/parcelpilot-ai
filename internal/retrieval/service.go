package retrieval

import (
	"context"
	"fmt"

	"github.com/Aditya7880900936/parcelpilot-ai/internal/embeddings"
)

type Service struct {
	embedder  embeddings.Provider
	retriever Retriever
}

func NewService(
	embedder embeddings.Provider,
	retriever Retriever,
) *Service {
	return &Service{
		embedder:  embedder,
		retriever: retriever,
	}
}

func (s *Service) Search(
	ctx context.Context,
	query string,
	limit int,
) ([]Chunk, error) {
	if query == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}

	vectors, err := s.embedder.Embed(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}

	if len(vectors) != 1 {
		return nil, fmt.Errorf("expected one query embedding")
	}

	chunks, err := s.retriever.Search(ctx, vectors[0], limit)
	if err != nil {
		return nil, fmt.Errorf("retrieve chunks: %w", err)
	}

	return chunks, nil
}
