package agent

import "github.com/Aditya7880900936/parcelpilot-ai/internal/domain"

type Context struct {
	Query   string
	Account *domain.AccountContext
	Order   *domain.OrderContext
	Tickets []domain.TicketContext
	Chunks  []Source
}
