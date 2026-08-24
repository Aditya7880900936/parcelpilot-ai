package agent

import "context"

type Agent interface {
	Answer(ctx context.Context, input string) (*Response, error)
}

type Response struct {
	Answer     string
	Confidence float64
	Sources    []Source
	Action     *Action
	Escalate   bool
	Reason     string
}

type Source struct {
	DocumentID int64
	Content    string
	Score      float64
}

type Action struct {
	Type   string
	Target string
	Reason string
}
