package db

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type DocumentRepository struct {
	tx pgx.Tx
}

func NewDocumentRepository(tx pgx.Tx) *DocumentRepository {
	return &DocumentRepository{tx: tx}
}

func (r *DocumentRepository) UpsertDocument(
	ctx context.Context,
	filename string,
	title string,
	documentType string,
	status string,
	authorityRank int,
	accountID *string,
) (int64, error) {

	var id int64

	metadata, err := json.Marshal(map[string]string{
		"source": "assessment_pdf",
	})
	if err != nil {
		return 0, fmt.Errorf("marshal document metadata: %w", err)
	}

	err = r.tx.QueryRow(ctx, `
		INSERT INTO documents (
			filename,
			title,
			document_type,
			status,
			authority_rank,
			account_id,
			metadata
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (filename)
		DO UPDATE SET
			title = EXCLUDED.title,
			document_type = EXCLUDED.document_type,
			status = EXCLUDED.status,
			authority_rank = EXCLUDED.authority_rank,
			account_id = EXCLUDED.account_id,
			metadata = EXCLUDED.metadata,
			updated_at = NOW()
		RETURNING id
	`,
		filename,
		title,
		documentType,
		status,
		authorityRank,
		accountID,
		metadata,
	).Scan(&id)

	if err != nil {
		return 0, fmt.Errorf("upsert document: %w", err)
	}

	return id, nil
}

func (r *DocumentRepository) ReplaceChunks(
	ctx context.Context,
	documentID int64,
	chunks []string,
) error {

	_, err := r.tx.Exec(ctx, `
		DELETE FROM document_chunks
		WHERE document_id = $1
	`, documentID)

	if err != nil {
		return fmt.Errorf("delete existing chunks: %w", err)
	}

	for i, content := range chunks {
		metadata, err := json.Marshal(map[string]any{
			"source":      "assessment_pdf",
			"chunk_index": i,
		})
		if err != nil {
			return fmt.Errorf("marshal chunk metadata: %w", err)
		}

		_, err = r.tx.Exec(ctx, `
			INSERT INTO document_chunks (
				document_id,
				chunk_index,
				content,
				metadata
			)
			VALUES ($1,$2,$3,$4)
		`,
			documentID,
			i,
			content,
			metadata,
		)

		if err != nil {
			return fmt.Errorf("insert chunk %d: %w", i, err)
		}
	}

	return nil
}
