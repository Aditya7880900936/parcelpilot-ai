package embeddings

import (
	"context"
	"fmt"
)

type Service struct {
	provider Provider
}

func NewService(provider Provider) *Service {
	return &Service{
		provider: provider,
	}
}

func (s *Service) Embed(
	ctx context.Context,
	texts []string,
) ([][]float32, error) {

	if len(texts) == 0 {
		return nil, nil
	}

	embeddings, err := s.provider.Embed(ctx, texts)
	if err != nil {
		return nil, fmt.Errorf("embed documents: %w", err)
	}

	if len(embeddings) != len(texts) {
		return nil, fmt.Errorf(
			"embedding count mismatch: got %d, expected %d",
			len(embeddings),
			len(texts),
		)
	}

	return embeddings, nil
}
