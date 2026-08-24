package document

import "errors"

var ErrUnknownDocument = errors.New("unknown document")

type Service struct {
	extractor *Extractor
	chunker   *Chunker
}

func NewService() *Service {
	return &Service{
		extractor: NewExtractor(),
		chunker:   NewChunker(1200),
	}
}

type ProcessedDocument struct {
	Filename string
	Metadata Metadata
	Chunks   []string
}

func (s *Service) Process(
	path string,
	filename string,
) (*ProcessedDocument, error) {

	text, err := s.extractor.Extract(path)
	if err != nil {
		return nil, err
	}

	meta, ok := MetadataFor(filename)
	if !ok {
		return nil, ErrUnknownDocument
	}

	chunks := s.chunker.Chunk(text)

	return &ProcessedDocument{
		Filename: filename,
		Metadata: meta,
		Chunks:   chunks,
	}, nil
}
