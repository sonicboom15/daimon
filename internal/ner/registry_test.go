// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package ner_test

import (
	"context"
	"testing"

	"github.com/sonicboom15/daimon/internal/ner"
)

type mockNER struct{}

func (m *mockNER) Extract(ctx context.Context, req ner.ExtractRequest) (*ner.ExtractResponse, error) {
	return &ner.ExtractResponse{
		Entities: []ner.Entity{
			{
				Text:       "asthma",
				Label:      "disease",
				Start:      0,
				End:        6,
				Confidence: 0.95,
			},
		},
	}, nil
}

func TestRegistry(t *testing.T) {
	ner.Register("mock", func(cfg ner.Config) (ner.Model, error) {
		return &mockNER{}, nil
	})

	if !ner.HasNER("mock") {
		t.Fatal("expected mock to be registered")
	}

	model, err := ner.New("mock", ner.Config{})
	if err != nil {
		t.Fatalf("unexpected error creating model: %v", err)
	}

	res, err := model.Extract(context.Background(), ner.ExtractRequest{Text: "asthma"})
	if err != nil {
		t.Fatalf("unexpected extract error: %v", err)
	}
	if len(res.Entities) != 1 || res.Entities[0].Text != "asthma" {
		t.Fatalf("unexpected entities: %+v", res.Entities)
	}

	_, err = ner.New("nonexistent", ner.Config{})
	if err == nil {
		t.Fatal("expected error for nonexistent model")
	}
}

