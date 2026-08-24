package agent

import (
	"context"
	"fmt"

	"github.com/Aditya7880900936/parcelpilot-ai/internal/domain"
)

type Retriever interface {
	Search(ctx context.Context, query string, limit int) ([]Source, error)
}

type ContextLoader interface {
	Load(
		ctx context.Context,
		accountID string,
		orderID string,
	) (*domain.AccountContext, *domain.OrderContext, []domain.TicketContext, error)
}

type Orchestrator struct {
	retriever      Retriever
	contextLoader  ContextLoader
	decisionEngine *DecisionEngine
	contextBuilder *ContextBuilder
}

func NewOrchestrator(
	retriever Retriever,
	contextLoader ContextLoader,
	decisionEngine *DecisionEngine,
	contextBuilder *ContextBuilder,
) *Orchestrator {
	return &Orchestrator{
		retriever:      retriever,
		contextLoader:  contextLoader,
		decisionEngine: decisionEngine,
		contextBuilder: contextBuilder,
	}
}

func (o *Orchestrator) Evaluate(
	ctx context.Context,
	query string,
	accountID string,
	orderID string,
) (*Response, error) {
	if query == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}

	chunks, err := o.retriever.Search(ctx, query, 5)
	if err != nil {
		return nil, fmt.Errorf("retrieve context: %w", err)
	}

	account, order, tickets, err := o.contextLoader.Load(
		ctx,
		accountID,
		orderID,
	)
	if err != nil {
		return nil, fmt.Errorf("load structured context: %w", err)
	}

	agentContext := o.contextBuilder.Build(
		account,
		order,
		tickets,
		chunks,
		query,
	)

	decision := o.decisionEngine.Evaluate(agentContext)

	return &Response{
		Answer:   decision.Reason,
		Sources:  chunks,
		Action:   decision.Action,
		Escalate: decision.Escalate,
		Reason:   decision.Reason,
	}, nil
}
