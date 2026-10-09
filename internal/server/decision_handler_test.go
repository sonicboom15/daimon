// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sonicboom15/daimon/internal/conversation"
	"github.com/sonicboom15/daimon/internal/decision"
	"github.com/sonicboom15/daimon/internal/mcp"
	"github.com/sonicboom15/daimon/internal/memory"
	"github.com/sonicboom15/daimon/internal/ner"
	"github.com/sonicboom15/daimon/internal/server"
	"github.com/sonicboom15/daimon/internal/session"
)

type mockDecisionModel struct{}

func (m *mockDecisionModel) Choose(ctx context.Context, req decision.ChoiceRequest) (*decision.ChoiceResponse, error) {
	return &decision.ChoiceResponse{
		Selected: "M25.561",
		Index:    0,
		Probabilities: map[string]float64{
			"M25.561": 0.97,
		},
	}, nil
}

func (m *mockDecisionModel) Verify(ctx context.Context, req decision.VerifyRequest) (*decision.VerifyResponse, error) {
	return &decision.VerifyResponse{
		Probability: 0.95,
		Supported:   true,
	}, nil
}

func TestDecisionHTTPHandlers(t *testing.T) {
	decisionModels := map[string]decision.Model{
		"verifier": &mockDecisionModel{},
	}

	srv := server.New(
		make(map[string]conversation.Conversation),
		[]*mcp.Client{},
		make(map[string]memory.MemoryStore),
		make(map[string]memory.GraphStore),
		make(map[string]ner.Model),
		decisionModels,
		make(map[string]string),
		session.NewInMemory(),
	)

	// Test Choose endpoint
	choosePayload := `{"question":"Which code?","choices":["M25.561"]}`
	chooseReq := httptest.NewRequest(http.MethodPost, "/v1/decision/verifier/choose", bytes.NewBufferString(choosePayload))
	chooseW := httptest.NewRecorder()
	srv.ServeHTTP(chooseW, chooseReq)

	if chooseW.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", chooseW.Code, chooseW.Body.String())
	}

	var chooseResp decision.ChoiceResponse
	if err := json.NewDecoder(chooseW.Body).Decode(&chooseResp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if chooseResp.Selected != "M25.561" {
		t.Fatalf("unexpected choose response: %+v", chooseResp)
	}

	// Test Verify endpoint
	verifyPayload := `{"statement":"Right knee pain"}`
	verifyReq := httptest.NewRequest(http.MethodPost, "/v1/decision/verifier/verify", bytes.NewBufferString(verifyPayload))
	verifyW := httptest.NewRecorder()
	srv.ServeHTTP(verifyW, verifyReq)

	if verifyW.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", verifyW.Code, verifyW.Body.String())
	}

	var verifyResp decision.VerifyResponse
	if err := json.NewDecoder(verifyW.Body).Decode(&verifyResp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if !verifyResp.Supported || verifyResp.Probability != 0.95 {
		t.Fatalf("unexpected verify response: %+v", verifyResp)
	}
}

