package agent

import "github.com/Aditya7880900936/parcelpilot-ai/internal/domain"

type ContextBuilder struct{}

func NewContextBuilder() *ContextBuilder {
	return &ContextBuilder{}
}

func (b *ContextBuilder) Build(
	account *domain.AccountContext,
	order *domain.OrderContext,
	tickets []domain.TicketContext,
	chunks []Source,
	query string,
) Context {
	return Context{
		Query:   query,
		Account: account,
		Order:   order,
		Tickets: tickets,
		Chunks:  chunks,
	}
}
