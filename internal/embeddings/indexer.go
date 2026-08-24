package embeddings

import (
	"context"
	"fmt"

	"github.com/Aditya7880900936/parcelpilot-ai/internal/db"
)

type Indexer struct {
	chunks  *db.ChunkRepository
	service *Service
}

func NewIndexer(
	chunks *db.ChunkRepository,
	service *Service,
) *Indexer {
	return &Indexer{
		chunks:  chunks,
		service: service,
	}
}

func (i *Indexer) Index(
	ctx context.Context,
	limit int,
) error {
	chunks, err := i.chunks.GetUnembedded(ctx, limit)
	if err != nil {
		return fmt.Errorf("get unembedded chunks: %w", err)
	}

	if len(chunks) == 0 {
		return nil
	}

	texts := make([]string, len(chunks))

	for idx, chunk := range chunks {
		texts[idx] = chunk.Content
	}

	vectors, err := i.service.Embed(ctx, texts)
	if err != nil {
		return fmt.Errorf("generate embeddings: %w", err)
	}

	for idx, chunk := range chunks {
		if err := i.chunks.UpdateEmbedding(
			ctx,
			chunk.ID,
			vectors[idx],
		); err != nil {
			return fmt.Errorf(
				"store embedding for chunk %d: %w",
				chunk.ID,
				err,
			)
		}
	}

	return nil
}
