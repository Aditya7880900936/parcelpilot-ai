package agent

import (
	"context"

	"github.com/Aditya7880900936/parcelpilot-ai/internal/retrieval"
)

type RetrieverAdapter struct {
	service *retrieval.Service
}

func NewRetrieverAdapter(service *retrieval.Service) *RetrieverAdapter {
	return &RetrieverAdapter{service: service}
}

func (r *RetrieverAdapter) Search(
	ctx context.Context,
	query string,
	limit int,
) ([]Source, error) {
	chunks, err := r.service.Search(ctx, query, limit)
	if err != nil {
		return nil, err
	}

	sources := make([]Source, 0, len(chunks))

	for _, chunk := range chunks {
		sources = append(sources, Source{
			DocumentID: chunk.DocumentID,
			Content:    chunk.Content,
			Score:      chunk.Score,
		})
	}

	return sources, nil
}
