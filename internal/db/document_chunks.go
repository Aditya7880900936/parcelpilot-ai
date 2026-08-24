package db

import (
	"context"
	"fmt"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Chunk struct {
	ID         int64
	DocumentID int64
	ChunkIndex int
	Content    string
}

type ChunkRepository struct {
	pool *pgxpool.Pool
}

func NewChunkRepository(pool *pgxpool.Pool) *ChunkRepository {
	return &ChunkRepository{pool: pool}
}

func (r *ChunkRepository) GetUnembedded(
	ctx context.Context,
	limit int,
) ([]Chunk, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, document_id, chunk_index, content
		FROM document_chunks
		WHERE embedding IS NULL
		ORDER BY id
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("query unembedded chunks: %w", err)
	}
	defer rows.Close()

	var chunks []Chunk

	for rows.Next() {
		var chunk Chunk

		if err := rows.Scan(
			&chunk.ID,
			&chunk.DocumentID,
			&chunk.ChunkIndex,
			&chunk.Content,
		); err != nil {
			return nil, fmt.Errorf("scan chunk: %w", err)
		}

		chunks = append(chunks, chunk)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate chunks: %w", err)
	}

	return chunks, nil
}

func (r *ChunkRepository) UpdateEmbedding(
	ctx context.Context,
	chunkID int64,
	embedding []float32,
) error {
	if len(embedding) != 768 {
		return fmt.Errorf(
			"invalid embedding dimension: got %d, expected 768",
			len(embedding),
		)
	}

	vector := make([]byte, 0, len(embedding)*8+2)
	vector = append(vector, '[')

	for i, value := range embedding {
		if i > 0 {
			vector = append(vector, ',')
		}

		vector = strconv.AppendFloat(
			vector,
			float64(value),
			'g',
			-1,
			32,
		)
	}

	vector = append(vector, ']')

	_, err := r.pool.Exec(ctx, `
		UPDATE document_chunks
		SET embedding = $1::vector
		WHERE id = $2
	`, string(vector), chunkID)

	if err != nil {
		return fmt.Errorf(
			"update embedding for chunk %d: %w",
			chunkID,
			err,
		)
	}

	return nil
}
