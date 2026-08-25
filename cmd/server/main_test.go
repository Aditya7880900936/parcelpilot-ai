package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthEndpoint(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"status": "ok",
		})
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	if !strings.Contains(rec.Body.String(), `"status":"ok"`) {
		t.Fatalf("unexpected response: %s", rec.Body.String())
	}
}

func TestHealthEndpointRejectsInvalidMethod(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
				"error": "method not allowed",
			})
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{
			"status": "ok",
		})
	})

	req := httptest.NewRequest(http.MethodPost, "/health", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf(
			"expected status 405, got %d",
			rec.Code,
		)
	}
}

func TestEvaluateEndpointRejectsInvalidJSON(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/evaluate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
				"error": "method not allowed",
			})
			return
		}

		var req evaluateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "invalid JSON body",
			})
			return
		}
	})

	req := httptest.NewRequest(
		http.MethodPost,
		"/evaluate",
		strings.NewReader(`{"query":`),
	)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestEvaluateEndpointRejectsMissingFields(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/evaluate", func(w http.ResponseWriter, r *http.Request) {
		var req evaluateRequest

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "invalid JSON body",
			})
			return
		}

		switch {
		case req.Query == "":
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "query is required",
			})
		case req.AccountID == "":
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "account_id is required",
			})
		case req.OrderID == "":
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "order_id is required",
			})
		}
	})

	tests := []struct {
		name string
		body string
	}{
		{
			name: "missing query",
			body: `{"account_id":"ACCT-001","order_id":"ORD-2002"}`,
		},
		{
			name: "missing account",
			body: `{"query":"Can I cancel?","order_id":"ORD-2002"}`,
		},
		{
			name: "missing order",
			body: `{"query":"Can I cancel?","account_id":"ACCT-001"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(
				http.MethodPost,
				"/evaluate",
				strings.NewReader(tt.body),
			)

			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected status 400, got %d", rec.Code)
			}
		})
	}
}
