package retrieval

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRetriever struct {
	pool *pgxpool.Pool
}

func NewPostgresRetriever(pool *pgxpool.Pool) *PostgresRetriever {
	return &PostgresRetriever{pool: pool}
}

func (r *PostgresRetriever) Search(
	ctx context.Context,
	embedding []float32,
	limit int,
) ([]Chunk, error) {
	if len(embedding) != 768 {
		return nil, fmt.Errorf(
			"invalid query embedding dimension: got %d, expected 768",
			len(embedding),
		)
	}

	if limit <= 0 {
		limit = 5
	}

	rows, err := r.pool.Query(ctx, `
		SELECT
			id,
			document_id,
			content,
			metadata,
			1 - (embedding <=> $1::vector) AS score
		FROM document_chunks
		WHERE embedding IS NOT NULL
		ORDER BY embedding <=> $1::vector
		LIMIT $2
	`, vectorLiteral(embedding), limit)
	if err != nil {
		return nil, fmt.Errorf("search document chunks: %w", err)
	}
	defer rows.Close()

	var chunks []Chunk

	for rows.Next() {
		var (
			chunk    Chunk
			metadata []byte
		)

		if err := rows.Scan(
			&chunk.ID,
			&chunk.DocumentID,
			&chunk.Content,
			&metadata,
			&chunk.Score,
		); err != nil {
			return nil, fmt.Errorf("scan document chunk: %w", err)
		}

		if err := json.Unmarshal(metadata, &chunk.Metadata); err != nil {
			return nil, fmt.Errorf("decode chunk metadata: %w", err)
		}

		chunks = append(chunks, chunk)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate document chunks: %w", err)
	}

	return chunks, nil
}

func vectorLiteral(values []float32) string {
	result := "["
	for i, value := range values {
		if i > 0 {
			result += ","
		}
		result += fmt.Sprintf("%f", value)
	}
	result += "]"

	return result
}
