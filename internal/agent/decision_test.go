// internal/agent/decision_test.go
package agent

import (
	"testing"
	"time"

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

func TestDecisionEngineServiceCreditEligible(t *testing.T) {
	engine := NewDecisionEngine()

	pickupEnd := time.Now().Add(-3 * time.Hour)

	ctx := Context{
		Account: &domain.AccountContext{
			AccountID:   "ACCT-001",
			AccountName: "Northstar Logistics",
		},
		Order: &domain.OrderContext{
			OrderID:         "ORD-CREDIT-001",
			Status:          "BOOKED",
			ShipmentFee:     1200,
			CarrierFault:    true,
			CustomerFault:   false,
			PickupWindowEnd: &pickupEnd,
		},
	}

	decision := engine.EvaluateServiceCredit(ctx, time.Now())

	if !decision.Allowed {
		t.Fatalf("expected service credit to be allowed: %s", decision.Reason)
	}

	if decision.Action == nil {
		t.Fatal("expected SERVICE_CREDIT action")
	}

	if decision.Action.Type != "SERVICE_CREDIT" {
		t.Fatalf("expected SERVICE_CREDIT, got %s", decision.Action.Type)
	}
}

func TestDecisionEngineServiceCreditCarrierFaultUnknown(t *testing.T) {
	engine := NewDecisionEngine()

	pickupEnd := time.Now().Add(-3 * time.Hour)

	ctx := Context{
		Account: &domain.AccountContext{
			AccountID:   "ACCT-001",
			AccountName: "Northstar Logistics",
		},
		Order: &domain.OrderContext{
			OrderID:         "ORD-CREDIT-002",
			Status:          "BOOKED",
			ShipmentFee:     1200,
			CarrierFault:    false,
			CustomerFault:   false,
			PickupWindowEnd: &pickupEnd,
		},
	}

	decision := engine.EvaluateServiceCredit(ctx, time.Now())

	if decision.Allowed {
		t.Fatal("expected credit to be denied")
	}

	if decision.Action != nil {
		t.Fatal("expected no action")
	}
}

func TestDecisionEngineServiceCreditCustomerFault(t *testing.T) {
	engine := NewDecisionEngine()

	pickupEnd := time.Now().Add(-3 * time.Hour)

	ctx := Context{
		Account: &domain.AccountContext{
			AccountID:   "ACCT-001",
			AccountName: "Northstar Logistics",
		},
		Order: &domain.OrderContext{
			OrderID:         "ORD-CREDIT-003",
			Status:          "BOOKED",
			ShipmentFee:     1200,
			CarrierFault:    true,
			CustomerFault:   true,
			PickupWindowEnd: &pickupEnd,
		},
	}

	decision := engine.EvaluateServiceCredit(ctx, time.Now())

	if decision.Allowed {
		t.Fatal("expected credit to be denied")
	}
}

func TestDecisionEngineLumenWorksServiceCreditOverride(t *testing.T) {
	engine := NewDecisionEngine()

	pickupEnd := time.Now().Add(-5 * time.Hour)

	ctx := Context{
		Account: &domain.AccountContext{
			AccountID:   "ACCT-002",
			AccountName: "LumenWorks",
		},
		Order: &domain.OrderContext{
			OrderID:         "ORD-CREDIT-004",
			Status:          "BOOKED",
			ShipmentFee:     5000,
			CarrierFault:    true,
			CustomerFault:   false,
			PickupWindowEnd: &pickupEnd,
		},
		Chunks: []Source{
			{
				Content: "LumenWorks receives a fixed INR 300 service credit if a pickup is more than 4 hours past the end of the scheduled pickup window, the carrier is at fault, and the customer is not at fault.",
				Score:   0.95,
			},
		},
	}

	decision := engine.EvaluateServiceCredit(ctx, time.Now())

	if !decision.Allowed {
		t.Fatalf("expected credit to be allowed: %s", decision.Reason)
	}

	if decision.Action == nil {
		t.Fatal("expected SERVICE_CREDIT action")
	}

	if decision.Action.Type != "SERVICE_CREDIT" {
		t.Fatalf("expected SERVICE_CREDIT, got %s", decision.Action.Type)
	}
}

func TestDecisionEngineServiceCreditMissingPickupWindow(t *testing.T) {
	engine := NewDecisionEngine()

	ctx := Context{
		Account: &domain.AccountContext{
			AccountID:   "ACCT-001",
			AccountName: "Northstar Logistics",
		},
		Order: &domain.OrderContext{
			OrderID:      "ORD-CREDIT-005",
			Status:       "BOOKED",
			ShipmentFee:  1200,
			CarrierFault: true,
		},
	}

	decision := engine.EvaluateServiceCredit(ctx, time.Now())

	if !decision.NeedsAgent {
		t.Fatal("expected NeedsAgent=true")
	}

	if decision.Action != nil {
		t.Fatal("expected no action")
	}
}
