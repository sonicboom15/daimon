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

type mockNERModel struct{}

func (m *mockNERModel) Extract(ctx context.Context, req ner.ExtractRequest) (*ner.ExtractResponse, error) {
	return &ner.ExtractResponse{
		Entities: []ner.Entity{
			{
				Text:       "diabetes",
				Label:      "disease",
				Start:      0,
				End:        8,
				Confidence: 0.98,
			},
		},
	}, nil
}

func TestNERHTTPHandler(t *testing.T) {
	nerModels := map[string]ner.Model{
		"clinical-ner": &mockNERModel{},
	}

	srv := server.New(
		make(map[string]conversation.Conversation),
		[]*mcp.Client{},
		make(map[string]memory.MemoryStore),
		make(map[string]memory.GraphStore),
		nerModels,
		make(map[string]decision.Model),
		make(map[string]string),
		session.NewInMemory(),
	)

	payload := `{"text":"diabetes"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/ner/clinical-ner/extract", bytes.NewBufferString(payload))
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp ner.ExtractResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}

	if len(resp.Entities) != 1 || resp.Entities[0].Text != "diabetes" {
		t.Fatalf("unexpected entities: %+v", resp.Entities)
	}
}

