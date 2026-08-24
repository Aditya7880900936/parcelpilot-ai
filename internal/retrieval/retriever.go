package retrieval

import "context"

type Chunk struct {
	ID         int64
	DocumentID int64
	Content    string
	Metadata   map[string]any
	Score      float64
}

type Retriever interface {
	Search(
		ctx context.Context,
		embedding []float32,
		limit int,
	) ([]Chunk, error)
}
