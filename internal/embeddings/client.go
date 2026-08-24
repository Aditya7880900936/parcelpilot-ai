package embeddings

import (
	"context"
	"fmt"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

type Provider interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

type OpenAIProvider struct {
	client *openai.Client
	model  string
}

func NewOpenAIProvider(apiKey string) *OpenAIProvider {
	client := openai.NewClient(
		option.WithAPIKey(apiKey),
	)

	return &OpenAIProvider{
		client: &client,
		model:  "text-embedding-3-small",
	}
}

func (p *OpenAIProvider) Embed(
	ctx context.Context,
	texts []string,
) ([][]float32, error) {

	if len(texts) == 0 {
		return nil, nil
	}

	input := make([]string, len(texts))
	copy(input, texts)

	response, err := p.client.Embeddings.New(ctx, openai.EmbeddingNewParams{
		Model: openai.EmbeddingModel(p.model),
		Input: openai.EmbeddingNewParamsInputUnion{
			OfArrayOfStrings: input,
		},
	})

	if err != nil {
		return nil, fmt.Errorf("create embeddings: %w", err)
	}

	result := make([][]float32, len(response.Data))

	for _, item := range response.Data {
		result[item.Index] = make([]float32, len(item.Embedding))

		for i, value := range item.Embedding {
			result[item.Index][i] = float32(value)
		}
	}

	return result, nil
}
