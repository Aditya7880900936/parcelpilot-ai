package domain

type AccountContext struct {
	AccountID      string
	AccountName    string
	Plan           string
	Status         string
	PremiumSupport bool
}

type OrderContext struct {
	OrderID       string
	AccountID     string
	Status        string
	Carrier       string
	ShipmentFee   float64
	CarrierFault  bool
	CustomerFault bool
}

type TicketContext struct {
	TicketID string
	Status   string
	Subject  string
}
