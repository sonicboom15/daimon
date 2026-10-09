// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package jev_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sonicboom15/daimon/internal/components/decision/jev"
	"github.com/sonicboom15/daimon/internal/decision"
)

func TestJevDriver(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		switch r.URL.Path {
		case "/choose":
			resp := decision.ChoiceResponse{
				Selected: "M25.561",
				Index:    0,
				Probabilities: map[string]float64{
					"M25.561": 0.97,
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
		case "/verify":
			resp := decision.VerifyResponse{
				Probability: 0.99,
				Supported:   true,
			}
			_ = json.NewEncoder(w).Encode(resp)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	client, err := jev.New(decision.Config{
		Metadata: map[string]string{
			"base_url": ts.URL,
			"api_key":  "test-key",
		},
	})
	if err != nil {
		t.Fatalf("unexpected init error: %v", err)
	}

	cRes, err := client.Choose(context.Background(), decision.ChoiceRequest{
		Question: "Which code?",
		Choices:  []string{"M25.561"},
	})
	if err != nil || cRes.Selected != "M25.561" {
		t.Fatalf("unexpected choose result: %v, %v", cRes, err)
	}

	vRes, err := client.Verify(context.Background(), decision.VerifyRequest{
		Statement: "Right knee pain",
	})
	if err != nil || !vRes.Supported {
		t.Fatalf("unexpected verify result: %v, %v", vRes, err)
	}
}

