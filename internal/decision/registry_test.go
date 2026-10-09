// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package decision_test

import (
	"context"
	"testing"

	"github.com/sonicboom15/daimon/internal/decision"
)

type mockDecision struct{}

func (m *mockDecision) Choose(ctx context.Context, req decision.ChoiceRequest) (*decision.ChoiceResponse, error) {
	return &decision.ChoiceResponse{
		Selected: "M25.561",
		Index:    0,
		Probabilities: map[string]float64{
			"M25.561": 0.95,
		},
	}, nil
}

func (m *mockDecision) Verify(ctx context.Context, req decision.VerifyRequest) (*decision.VerifyResponse, error) {
	return &decision.VerifyResponse{
		Probability: 0.98,
		Supported:   true,
	}, nil
}

func TestRegistry(t *testing.T) {
	decision.Register("mock", func(cfg decision.Config) (decision.Model, error) {
		return &mockDecision{}, nil
	})

	if !decision.HasDecision("mock") {
		t.Fatal("expected mock to be registered")
	}

	model, err := decision.New("mock", decision.Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	choiceRes, err := model.Choose(context.Background(), decision.ChoiceRequest{
		Question: "Which code?",
		Choices:  []string{"M25.561"},
	})
	if err != nil || choiceRes.Selected != "M25.561" {
		t.Fatalf("unexpected choose result: %v, %v", choiceRes, err)
	}

	verifyRes, err := model.Verify(context.Background(), decision.VerifyRequest{
		Statement: "Right knee pain",
	})
	if err != nil || !verifyRes.Supported {
		t.Fatalf("unexpected verify result: %v, %v", verifyRes, err)
	}

	_, err = decision.New("nonexistent", decision.Config{})
	if err == nil {
		t.Fatal("expected error for nonexistent model")
	}
}

