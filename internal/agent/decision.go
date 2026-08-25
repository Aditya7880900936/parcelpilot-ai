package agent

import (
	"fmt"
	"strings"
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

	// Security incidents / suspected credential exposure are P1.
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
			Reason:     "order context is required for cancellation evaluation",
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
	}

	return Decision{
		NeedsAgent: true,
		Confidence: 0,
		Reason: fmt.Sprintf(
			"no deterministic rule for order status %s",
			ctx.Order.Status,
		),
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

		// Customer-specific agreement allowing cancellation.
		if strings.Contains(content, accountName) &&
			strings.Contains(content, "may cancel any booked shipment") &&
			strings.Contains(content, "before pickup") &&
			strings.Contains(content, "no cancellation fee") {

			hasAgreement = true
			agreementCount++
		}

		// Customer-specific agreement explicitly denying cancellation.
		if strings.Contains(content, accountName) &&
			(strings.Contains(content, "cannot cancel") ||
				strings.Contains(content, "may not cancel") ||
				strings.Contains(content, "cancellation is not permitted")) {

			hasDenyAgreement = true
			denyAgreementCount++
		}

		// Current default cancellation policy.
		if strings.Contains(content, "booked, not yet picked_up") &&
			strings.Contains(content, "may be cancelled") {

			hasPolicy = true
		}
	}

	// Conflicting customer-specific evidence is never auto-actioned.
	if hasAgreement && hasDenyAgreement {
		return Decision{
			NeedsAgent: true,
			Escalate:   true,
			Confidence: 0.0,
			Reason: fmt.Sprintf(
				"conflicting customer agreements found for %s; cancellation of order %s requires verification",
				ctx.Account.AccountName,
				ctx.Order.OrderID,
			),
		}
	}

	// Multiple contradictory agreement signals should also be treated
	// conservatively rather than executing a state-changing action.
	if agreementCount > 1 && denyAgreementCount > 0 {
		return Decision{
			NeedsAgent: true,
			Escalate:   true,
			Confidence: 0.0,
			Reason: fmt.Sprintf(
				"conflicting cancellation evidence found for %s; state-changing action requires verification",
				ctx.Account.AccountName,
			),
		}
	}

	// Strong customer agreement evidence.
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

	// Agreement exists, but retrieval confidence is insufficient.
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

	// Only default policy was found.
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
