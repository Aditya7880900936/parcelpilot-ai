package agent

import (
	"context"
	"testing"

	"github.com/Aditya7880900936/parcelpilot-ai/internal/domain"
)

type fakeRetriever struct {
	sources []Source
}

func (f *fakeRetriever) Search(
	ctx context.Context,
	query string,
	limit int,
) ([]Source, error) {
	return f.sources, nil
}

type fakeContextLoader struct {
	account *domain.AccountContext
	order   *domain.OrderContext
	tickets []domain.TicketContext
}

func (f *fakeContextLoader) Load(
	ctx context.Context,
	accountID string,
	orderID string,
) (*domain.AccountContext, *domain.OrderContext, []domain.TicketContext, error) {
	return f.account, f.order, f.tickets, nil
}

func TestOrchestratorEvaluateCancellation(t *testing.T) {
	account := &domain.AccountContext{
		AccountID:   "ACCT-001",
		AccountName: "Northstar Logistics",
	}

	order := &domain.OrderContext{
		OrderID: "TEST-CANCEL",
		Status:  "BOOKED",
	}

	retriever := &fakeRetriever{
		sources: []Source{
			{
				DocumentID: 5,
				Score:      0.80,
				Content: `ParcelPilot - Northstar Logistics Enterprise Agreement
Northstar may cancel any BOOKED shipment before pickup with no cancellation fee.`,
			},
		},
	}

	loader := &fakeContextLoader{
		account: account,
		order:   order,
	}

	orchestrator := NewOrchestrator(
		retriever,
		loader,
		NewDecisionEngine(),
		NewContextBuilder(),
	)

	response, err := orchestrator.Evaluate(
		context.Background(),
		"Can Northstar cancel TEST-CANCEL?",
		"ACCT-001",
		"TEST-CANCEL",
	)

	if err != nil {
		t.Fatalf("evaluate failed: %v", err)
	}

	if response.Action == nil {
		t.Fatal("expected cancellation action")
	}

	if response.Action.Type != "CANCEL_ORDER" {
		t.Fatalf("expected CANCEL_ORDER, got %s", response.Action.Type)
	}

	if response.Action.Target != "TEST-CANCEL" {
		t.Fatalf("expected TEST-CANCEL, got %s", response.Action.Target)
	}

	if !response.Escalate {
		if response.Confidence < 0.90 {
			t.Fatalf("expected high confidence, got %.2f", response.Confidence)
		}
	}
}

func TestOrchestratorEvaluatePickedUpOrder(t *testing.T) {
	account := &domain.AccountContext{
		AccountID:   "ACCT-001",
		AccountName: "Northstar Logistics",
	}

	order := &domain.OrderContext{
		OrderID: "TEST-RTO",
		Status:  "PICKED_UP",
	}

	orchestrator := NewOrchestrator(
		&fakeRetriever{},
		&fakeContextLoader{
			account: account,
			order:   order,
		},
		NewDecisionEngine(),
		NewContextBuilder(),
	)

	response, err := orchestrator.Evaluate(
		context.Background(),
		"Can Northstar cancel TEST-RTO?",
		"ACCT-001",
		"TEST-RTO",
	)

	if err != nil {
		t.Fatalf("evaluate failed: %v", err)
	}

	if response.Action == nil {
		t.Fatal("expected return-to-origin action")
	}

	if response.Action.Type != "RETURN_TO_ORIGIN" {
		t.Fatalf(
			"expected RETURN_TO_ORIGIN, got %s",
			response.Action.Type,
		)
	}
}

func TestOrchestratorRejectsEmptyQuery(t *testing.T) {
	orchestrator := NewOrchestrator(
		&fakeRetriever{},
		&fakeContextLoader{},
		NewDecisionEngine(),
		NewContextBuilder(),
	)

	_, err := orchestrator.Evaluate(
		context.Background(),
		"",
		"ACCT-001",
		"TEST-CANCEL",
	)

	if err == nil {
		t.Fatal("expected empty query to fail")
	}
}
