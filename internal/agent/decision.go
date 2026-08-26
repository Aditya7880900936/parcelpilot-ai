// internal/agent/decision.go
package agent

import (
	"fmt"
	"strings"
	"time"
)

type Decision struct {
	Allowed    bool
	NeedsAgent bool
	Escalate   bool
	Confidence float64
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
			Confidence: 0,
			Reason:     "account context is required",
		}
	}

	for _, ticket := range ctx.Tickets {
		if strings.EqualFold(ticket.Subject, "Possible API key exposure") {
			return Decision{
				Escalate:   true,
				Confidence: 1.0,
				Reason:     "suspected production credential exposure requires P1 handling",
				Action: &Action{
					Type:   "ESCALATE",
					Target: ticket.TicketID,
					Reason: "suspected API key exposure",
				},
			}
		}
	}

	if ctx.Order == nil {
		return Decision{
			NeedsAgent: true,
			Confidence: 0,
			Reason:     "order context is required",
		}
	}

	switch ctx.Order.Status {
	case "PICKED_UP":
		return Decision{
			Allowed:    false,
			Confidence: 1.0,
			Reason:     "picked-up shipments cannot be cancelled; use return-to-origin",
			Action: &Action{
				Type:   "RETURN_TO_ORIGIN",
				Target: ctx.Order.OrderID,
				Reason: "shipment already picked up",
			},
		}

	case "RETURN_TO_ORIGIN":
		return Decision{
			Allowed:    false,
			Confidence: 1.0,
			Reason:     "order is already marked for return-to-origin",
		}

	case "DELIVERED":
		return Decision{
			Allowed:    false,
			Confidence: 1.0,
			Reason:     "delivered shipments cannot be cancelled",
		}

	case "CANCELLED":
		return Decision{
			Allowed:    false,
			Confidence: 1.0,
			Reason:     "order is already cancelled",
		}

	case "BOOKED":
		return d.evaluateBookedCancellation(ctx)

	default:
		return Decision{
			NeedsAgent: true,
			Confidence: 0,
			Reason: fmt.Sprintf(
				"no deterministic rule for order status %s",
				ctx.Order.Status,
			),
		}
	}
}

func (d *DecisionEngine) EvaluateServiceCredit(
	ctx Context,
	now time.Time,
) Decision {
	if ctx.Account == nil {
		return Decision{
			NeedsAgent: true,
			Confidence: 0,
			Reason:     "account context is required",
		}
	}

	if ctx.Order == nil {
		return Decision{
			NeedsAgent: true,
			Confidence: 0,
			Reason:     "order context is required for service-credit evaluation",
		}
	}

	order := ctx.Order

	if order.PickupWindowEnd == nil {
		return Decision{
			NeedsAgent: true,
			Confidence: 1.0,
			Reason:     "pickup window end is unknown; service-credit eligibility cannot be verified",
		}
	}

	if now.Before(order.PickupWindowEnd.Add(2 * time.Hour)) {
		return Decision{
			Allowed:    false,
			Confidence: 1.0,
			Reason:     "pickup is not more than 2 hours past the scheduled pickup window",
		}
	}

	if !order.CarrierFault {
		return Decision{
			Allowed:    false,
			Confidence: 1.0,
			Reason:     "carrier fault is not confirmed",
		}
	}

	if order.CustomerFault {
		return Decision{
			Allowed:    false,
			Confidence: 1.0,
			Reason:     "customer-caused issue makes the order ineligible for a service credit",
		}
	}

	credit := order.ShipmentFee * 0.10

	if credit > 500 {
		credit = 500
	}

	threshold := 2 * time.Hour
	agreementCredit := false

	accountName := strings.ToLower(ctx.Account.AccountName)

	for _, chunk := range ctx.Chunks {
		content := normalizeText(chunk.Content)

		if !strings.Contains(content, accountName) {
			continue
		}

		if strings.Contains(content, "fixed inr 300 service credit") &&
			strings.Contains(content, "more than 4 hours past the end") &&
			strings.Contains(content, "carrier is at fault") {

			threshold = 4 * time.Hour
			credit = 300
			agreementCredit = true
		}
	}

	if now.Before(order.PickupWindowEnd.Add(threshold)) {
		return Decision{
			Allowed:    false,
			Confidence: 1.0,
			Reason: fmt.Sprintf(
				"pickup is not more than %s past the scheduled pickup window",
				threshold,
			),
		}
	}

	if credit > 1000 {
		return Decision{
			NeedsAgent: true,
			Confidence: 1.0,
			Reason: fmt.Sprintf(
				"service credit of INR %.2f requires manager approval",
				credit,
			),
			Action: &Action{
				Type:   "SERVICE_CREDIT_APPROVAL",
				Target: order.OrderID,
				Reason: fmt.Sprintf("manager approval required for INR %.2f service credit", credit),
			},
		}
	}

	reason := fmt.Sprintf(
		"order %s qualifies for INR %.2f service credit",
		order.OrderID,
		credit,
	)

	if agreementCredit {
		reason = fmt.Sprintf(
			"%s under the active customer agreement",
			reason,
		)
	}

	return Decision{
		Allowed:    true,
		Confidence: 1.0,
		Reason:     reason,
		Action: &Action{
			Type:   "SERVICE_CREDIT",
			Target: order.OrderID,
			Reason: reason,
		},
	}
}

func (d *DecisionEngine) evaluateBookedCancellation(ctx Context) Decision {
	var (
		hasAgreement       bool
		hasDenyAgreement   bool
		hasPolicy          bool
		bestScore          float64
		agreementCount     int
		denyAgreementCount int
	)

	accountName := strings.ToLower(ctx.Account.AccountName)

	for _, chunk := range ctx.Chunks {
		content := normalizeText(chunk.Content)

		if chunk.Score > bestScore {
			bestScore = chunk.Score
		}

		if strings.Contains(content, accountName) &&
			strings.Contains(content, "may cancel any booked shipment") &&
			strings.Contains(content, "before pickup") &&
			strings.Contains(content, "no cancellation fee") {
			hasAgreement = true
			agreementCount++
		}

		if strings.Contains(content, accountName) &&
			(strings.Contains(content, "cannot cancel") ||
				strings.Contains(content, "may not cancel") ||
				strings.Contains(content, "cancellation is not permitted")) {
			hasDenyAgreement = true
			denyAgreementCount++
		}

		if strings.Contains(content, "booked, not yet picked_up") &&
			strings.Contains(content, "may be cancelled") {
			hasPolicy = true
		}
	}

	if hasAgreement && hasDenyAgreement {
		return Decision{
			NeedsAgent: true,
			Escalate:   true,
			Confidence: 0,
			Reason: fmt.Sprintf(
				"conflicting customer agreements found for %s; cancellation of order %s requires verification",
				ctx.Account.AccountName,
				ctx.Order.OrderID,
			),
		}
	}

	if agreementCount > 1 && denyAgreementCount > 0 {
		return Decision{
			NeedsAgent: true,
			Escalate:   true,
			Confidence: 0,
			Reason: fmt.Sprintf(
				"conflicting cancellation evidence found for %s; state-changing action requires verification",
				ctx.Account.AccountName,
			),
		}
	}

	if hasAgreement && bestScore >= 0.60 {
		return Decision{
			Allowed:    true,
			Confidence: 0.95,
			Reason: fmt.Sprintf(
				"%s may cancel BOOKED order %s before pickup with no cancellation fee under the active customer agreement",
				ctx.Account.AccountName,
				ctx.Order.OrderID,
			),
			Action: &Action{
				Type:   "CANCEL_ORDER",
				Target: ctx.Order.OrderID,
				Reason: "active customer agreement overrides the default cancellation fee policy",
			},
		}
	}

	if hasAgreement {
		return Decision{
			NeedsAgent: true,
			Confidence: bestScore,
			Reason: fmt.Sprintf(
				"customer agreement appears to permit cancellation of BOOKED order %s, but evidence confidence is insufficient for an automatic action",
				ctx.Order.OrderID,
			),
		}
	}

	if hasPolicy {
		return Decision{
			NeedsAgent: true,
			Confidence: bestScore,
			Reason: fmt.Sprintf(
				"BOOKED order %s may be cancellable under the default policy, but no customer-specific agreement was verified",
				ctx.Order.OrderID,
			),
		}
	}

	return Decision{
		NeedsAgent: true,
		Confidence: bestScore,
		Reason: fmt.Sprintf(
			"BOOKED order %s requires policy/agreement evaluation",
			ctx.Order.OrderID,
		),
	}
}

func normalizeText(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(s)), " ")
}
