// internal/domain/context.go
package domain

import "time"

type AccountContext struct {
	AccountID      string
	AccountName    string
	Plan           string
	Status         string
	PremiumSupport bool
}

type OrderContext struct {
	OrderID         string
	AccountID       string
	Status          string
	Carrier         string
	ShipmentFee     float64
	CarrierFault    bool
	CustomerFault   bool
	PickupWindowEnd *time.Time
}

type TicketContext struct {
	TicketID string
	Status   string
	Subject  string
}
