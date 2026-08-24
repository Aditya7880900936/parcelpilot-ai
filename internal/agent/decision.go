package agent

import "fmt"

type Decision struct {
	Allowed    bool
	NeedsAgent bool
	Escalate   bool
	Reason     string
	Action     *Action
}

type DecisionEngine struct{}

func NewDecisionEngine() *DecisionEngine {
	return &DecisionEngine{}
}

func (d *DecisionEngine) Evaluate(ctx Context) Decision {
	if ctx.Account == nil {
		return Decision{
			NeedsAgent: true,
			Reason:     "account context is required",
		}
	}

	// Security incidents / suspected credential exposure are P1.
	for _, ticket := range ctx.Tickets {
		if ticket.Subject == "Possible API key exposure" {
			return Decision{
				Escalate: true,
				Reason:   "suspected production credential exposure requires P1 handling",
				Action: &Action{
					Type:   "ESCALATE",
					Target: ticket.TicketID,
					Reason: "suspected API key exposure",
				},
			}
		}
	}

	// Order-specific cancellation decision.
	if ctx.Order != nil {
		switch ctx.Order.Status {
		case "PICKED_UP":
			return Decision{
				Allowed: false,
				Reason:  "picked-up shipments cannot be cancelled; use return-to-origin",
				Action: &Action{
					Type:   "RETURN_TO_ORIGIN",
					Target: ctx.Order.OrderID,
					Reason: "shipment already picked up",
				},
			}

		case "DELIVERED":
			return Decision{
				Allowed: false,
				Reason:  "delivered shipments cannot be cancelled",
			}

		case "BOOKED":
			// Account-specific agreements are retrieved as context.
			return Decision{
				Allowed:    true,
				NeedsAgent: true,
				Reason: fmt.Sprintf(
					"BOOKED order %s requires policy/agreement evaluation",
					ctx.Order.OrderID,
				),
			}
		}
	}

	return Decision{
		NeedsAgent: true,
		Reason:     "deterministic rule not sufficient; agent reasoning required",
	}
}
