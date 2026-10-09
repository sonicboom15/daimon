// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	decisionhttp "github.com/sonicboom15/daimon/internal/components/decision/http"
	"github.com/sonicboom15/daimon/internal/decision"
)

func TestDecisionHTTPDriver(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/choose":
			var req decision.ChoiceRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			resp := decision.ChoiceResponse{
				Selected: req.Choices[0],
				Index:    0,
				Probabilities: map[string]float64{
					req.Choices[0]: 0.96,
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
		case "/verify":
			var req decision.VerifyRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			resp := decision.VerifyResponse{
				Probability: 0.89,
				Supported:   true,
			}
			_ = json.NewEncoder(w).Encode(resp)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	model, err := decisionhttp.New(decision.Config{
		Metadata: map[string]string{
			"base_url": ts.URL,
		},
	})
	if err != nil {
		t.Fatalf("unexpected init error: %v", err)
	}

	cRes, err := model.Choose(context.Background(), decision.ChoiceRequest{
		Question: "Which code?",
		Choices:  []string{"M25.561", "M25.562"},
	})
	if err != nil || cRes.Selected != "M25.561" {
		t.Fatalf("unexpected choose result: %v, %v", cRes, err)
	}

	vRes, err := model.Verify(context.Background(), decision.VerifyRequest{
		Statement: "Right knee effusion",
	})
	if err != nil || !vRes.Supported || vRes.Probability != 0.89 {
		t.Fatalf("unexpected verify result: %v, %v", vRes, err)
	}
}

