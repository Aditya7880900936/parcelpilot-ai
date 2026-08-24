package embeddings

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type OllamaProvider struct {
	baseURL string
	model   string
	client  *http.Client
}

func NewOllamaProvider(baseURL, model string) *OllamaProvider {
	return &OllamaProvider{
		baseURL: baseURL,
		model:   model,
		client:  &http.Client{},
	}
}

type ollamaEmbedRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type ollamaEmbedResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
}

func (p *OllamaProvider) Embed(
	ctx context.Context,
	texts []string,
) ([][]float32, error) {

	result := make([][]float32, 0, len(texts))

	for _, text := range texts {
		reqBody := ollamaEmbedRequest{
			Model: p.model,
			Input: text,
		}

		body, err := json.Marshal(reqBody)
		if err != nil {
			return nil, fmt.Errorf("marshal ollama request: %w", err)
		}

		req, err := http.NewRequestWithContext(
			ctx,
			http.MethodPost,
			p.baseURL+"/api/embed",
			bytes.NewReader(body),
		)
		if err != nil {
			return nil, fmt.Errorf("create ollama request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")

		resp, err := p.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("ollama request: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf(
				"ollama returned status %d",
				resp.StatusCode,
			)
		}

		var response ollamaEmbedResponse

		err = json.NewDecoder(resp.Body).Decode(&response)
		resp.Body.Close()

		if err != nil {
			return nil, fmt.Errorf("decode ollama response: %w", err)
		}

		if len(response.Embeddings) != 1 ||
			len(response.Embeddings[0]) != 768 {
			return nil, fmt.Errorf(
				"invalid ollama embedding dimension: expected 768",
			)
		}

		result = append(result, response.Embeddings[0])
	}

	return result, nil
}
