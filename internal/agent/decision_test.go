package agent

import (
	"testing"

	"github.com/Aditya7880900936/parcelpilot-ai/internal/domain"
)

func TestDecisionEngineReturnToOrigin(t *testing.T) {
	engine := NewDecisionEngine()

	ctx := Context{
		Account: &domain.AccountContext{
			AccountID:   "ACCT-001",
			AccountName: "Northstar Logistics",
		},
		Order: &domain.OrderContext{
			OrderID: "ORD-1002",
			Status:  "PICKED_UP",
		},
	}

	decision := engine.Evaluate(ctx)

	if decision.Action == nil {
		t.Fatal("expected RETURN_TO_ORIGIN action")
	}

	if decision.Action.Type != "RETURN_TO_ORIGIN" {
		t.Fatalf("expected RETURN_TO_ORIGIN, got %s", decision.Action.Type)
	}
}

func TestDecisionEngineAlreadyReturned(t *testing.T) {
	engine := NewDecisionEngine()

	ctx := Context{
		Account: &domain.AccountContext{
			AccountID:   "ACCT-001",
			AccountName: "Northstar Logistics",
		},
		Order: &domain.OrderContext{
			OrderID: "ORD-1002",
			Status:  "RETURN_TO_ORIGIN",
		},
	}

	decision := engine.Evaluate(ctx)

	if decision.Action != nil {
		t.Fatal("expected no action for already returned order")
	}

	if decision.Confidence != 1.0 {
		t.Fatalf("expected confidence 1.0, got %.2f", decision.Confidence)
	}
}
