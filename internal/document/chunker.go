package document

import "strings"

type Chunker struct {
	MaxChars int
}

func NewChunker(maxChars int) *Chunker {
	return &Chunker{
		MaxChars: maxChars,
	}
}

func (c *Chunker) Chunk(text string) []string {
	paragraphs := strings.Split(text, "\n")

	var chunks []string
	var current strings.Builder

	for _, paragraph := range paragraphs {
		paragraph = strings.TrimSpace(paragraph)

		if paragraph == "" {
			continue
		}

		if current.Len() > 0 &&
			current.Len()+1+len(paragraph) > c.MaxChars {

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
