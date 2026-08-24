package db

import (
	"context"
	"fmt"

	"github.com/Aditya7880900936/parcelpilot-ai/internal/domain"
)

type ContextLoader struct {
	repository *AccountContextRepository
}

func NewContextLoader(repository *AccountContextRepository) *ContextLoader {
	return &ContextLoader{
		repository: repository,
	}
}

func (l *ContextLoader) Load(
	ctx context.Context,
	accountID string,
	orderID string,
) (*domain.AccountContext, *domain.OrderContext, []domain.TicketContext, error) {
	account, err := l.repository.GetAccount(ctx, accountID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("load account: %w", err)
	}

	var order *domain.OrderContext

	if orderID != "" {
		order, err = l.repository.GetOrder(ctx, orderID)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("load order: %w", err)
		}
	}

	tickets, err := l.repository.GetTickets(ctx, accountID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("load tickets: %w", err)
	}

	return account, order, tickets, nil
}
