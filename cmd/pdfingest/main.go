package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
	"github.com/ledongthuc/pdf"

	"github.com/Aditya7880900936/parcelpilot-ai/internal/db"
)

type documentMeta struct {
	Title          string
	DocumentType   string
	Status         string
	AuthorityRank  int
	AccountID      *string
	EffectiveFrom  *time.Time
	EffectiveTo    *time.Time
	SourceUpdated  *time.Time
}

var documentMetadata = map[string]documentMeta{
	"01_Support_Policy_v3_CURRENT.pdf": {
		Title:         "ParcelPilot Support Policy v3",
		DocumentType:  "support_policy",
		Status:        "CURRENT",
		AuthorityRank: 2,
	},
	"02_Support_Policy_v2_DEPRECATED.pdf": {
		Title:         "ParcelPilot Support Policy v2",
		DocumentType:  "support_policy",
		Status:        "DEPRECATED",
		AuthorityRank: 4,
	},
	"03_Cancellation_and_Service_Credit_SOP_v4.pdf": {
		Title:         "Cancellation & Service Credit SOP v4",
		DocumentType:  "cancellation_sop",
		Status:        "CURRENT",
		AuthorityRank: 2,
	},
	"04_Product_Operations_Guide_and_Known_Issues.pdf": {
		Title:         "Product Operations Guide & Known Issues",
		DocumentType:  "product_documentation",
		Status:        "CURRENT",
		AuthorityRank: 3,
	},
	"05_Northstar_Logistics_Enterprise_Agreement.pdf": {
		Title:         "Northstar Logistics Enterprise Agreement",
		DocumentType:  "customer_agreement",
		Status:        "ACTIVE",
		AuthorityRank: 1,
		AccountID:     stringPtr("ACCT-001"),
	},
	"06_LumenWorks_Service_Agreement.pdf": {
		Title:         "LumenWorks Service Agreement",
		DocumentType:  "customer_agreement",
		Status:        "ACTIVE",
		AuthorityRank: 1,
		AccountID:     stringPtr("ACCT-002"),
	},
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("warning: .env not found, using environment variables")
	}

	ctx := context.Background()

	pool, err := db.NewPostgres(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	entries, err := os.ReadDir("data/pdfs")
	if err != nil {
		log.Fatal(err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".pdf") {
			continue
		}

		meta, ok := documentMetadata[entry.Name()]
		if !ok {
			log.Printf("skipping unknown PDF: %s", entry.Name())
			continue
		}

		path := filepath.Join("data/pdfs", entry.Name())

		text, err := extractText(path)
		if err != nil {
			log.Fatalf("extract %s: %v", entry.Name(), err)
		}

		chunks := chunkText(text, 1200)

		tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			log.Fatal(err)
		}

		var documentID int64

		err = tx.QueryRow(ctx, `
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
				metadata = EXCLUDED.metadata
			RETURNING id
		`,
			entry.Name(),
			meta.Title,
			meta.DocumentType,
			meta.Status,
			meta.AuthorityRank,
			meta.AccountID,
			`{"source":"assessment_pdf"}`,
		).Scan(&documentID)

		if err != nil {
			tx.Rollback(ctx)
			log.Fatalf("insert document %s: %v", entry.Name(), err)
		}

		_, err = tx.Exec(ctx, `
			DELETE FROM document_chunks
			WHERE document_id = $1
		`, documentID)

		if err != nil {
			tx.Rollback(ctx)
			log.Fatalf("clear chunks %s: %v", entry.Name(), err)
		}

		for i, chunk := range chunks {
			_, err = tx.Exec(ctx, `
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
				chunk,
				`{"source":"assessment_pdf"}`,
			)

			if err != nil {
				tx.Rollback(ctx)
				log.Fatalf(
					"insert chunk %s [%d]: %v",
					entry.Name(),
					i,
					err,
				)
			}
		}

		if err := tx.Commit(ctx); err != nil {
			log.Fatal(err)
		}

		log.Printf(
			"✓ %s → document=%d chunks=%d",
			entry.Name(),
			documentID,
			len(chunks),
		)
	}
}

func extractText(path string) (string, error) {
	f, reader, err := pdf.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var builder strings.Builder

	for page := 1; page <= reader.NumPage(); page++ {
		p := reader.Page(page)

		if p.V.IsNull() {
			continue
		}

		text, err := p.GetPlainText(nil)
		if err != nil {
			return "", fmt.Errorf("page %d: %w", page, err)
		}

		builder.WriteString(text)
		builder.WriteString("\n")
	}

	return normalizeText(builder.String()), nil
}

func normalizeText(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	lines := strings.Split(text, "\n")

	var cleaned []string

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if line != "" {
			cleaned = append(cleaned, line)
		}
	}

	return strings.Join(cleaned, "\n")
}

func chunkText(text string, maxChars int) []string {
	paragraphs := strings.Split(text, "\n")

	var chunks []string
	var current strings.Builder

	for _, paragraph := range paragraphs {
		paragraph = strings.TrimSpace(paragraph)

		if paragraph == "" {
			continue
		}

		if current.Len() > 0 &&
			current.Len()+1+len(paragraph) > maxChars {

			chunks = append(chunks, strings.TrimSpace(current.String()))
			current.Reset()
		}

		if current.Len() > 0 {
			current.WriteString("\n")
		}

		current.WriteString(paragraph)
	}

	if current.Len() > 0 {
		chunks = append(chunks, strings.TrimSpace(current.String()))
	}

	return chunks
}

func stringPtr(value string) *string {
	return &value
}