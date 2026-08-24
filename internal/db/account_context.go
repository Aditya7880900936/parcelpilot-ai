package db

import (
	"context"
	"fmt"

	"github.com/Aditya7880900936/parcelpilot-ai/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AccountContextRepository struct {
	pool *pgxpool.Pool
}

func NewAccountContextRepository(pool *pgxpool.Pool) *AccountContextRepository {
	return &AccountContextRepository{pool: pool}
}

func (r *AccountContextRepository) GetAccount(
	ctx context.Context,
	accountID string,
) (*domain.AccountContext, error) {
	var account domain.AccountContext

	err := r.pool.QueryRow(ctx, `
		SELECT
			account_id,
			account_name,
			plan,
			status,
			premium_support
		FROM accounts
		WHERE account_id = $1
	`, accountID).Scan(
		&account.AccountID,
		&account.AccountName,
		&account.Plan,
		&account.Status,
		&account.PremiumSupport,
	)

	if err != nil {
		return nil, fmt.Errorf("get account %s: %w", accountID, err)
	}

	return &account, nil
}

func (r *AccountContextRepository) GetOrder(
	ctx context.Context,
	orderID string,
) (*domain.OrderContext, error) {
	var order domain.OrderContext

	err := r.pool.QueryRow(ctx, `
		SELECT
			order_id,
			account_id,
			status,
			carrier,
			shipment_fee_inr,
			carrier_fault,
			customer_fault
		FROM orders
		WHERE order_id = $1
	`, orderID).Scan(
		&order.OrderID,
		&order.AccountID,
		&order.Status,
		&order.Carrier,
		&order.ShipmentFee,
		&order.CarrierFault,
		&order.CustomerFault,
	)

	if err != nil {
		return nil, fmt.Errorf("get order %s: %w", orderID, err)
	}

	return &order, nil
}

func (r *AccountContextRepository) GetTickets(
	ctx context.Context,
	accountID string,
) ([]domain.TicketContext, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT
			ticket_id,
			status,
			subject
		FROM tickets
		WHERE account_id = $1
		ORDER BY created_at DESC
	`, accountID)
	if err != nil {
		return nil, fmt.Errorf("get tickets for account %s: %w", accountID, err)
	}
	defer rows.Close()

	var tickets []domain.TicketContext

	for rows.Next() {
		var ticket domain.TicketContext

		if err := rows.Scan(
			&ticket.TicketID,
			&ticket.Status,
			&ticket.Subject,
		); err != nil {
			return nil, fmt.Errorf("scan ticket: %w", err)
		}

		tickets = append(tickets, ticket)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tickets: %w", err)
	}

	return tickets, nil
}
