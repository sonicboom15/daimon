// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package memgraph_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sonicboom15/daimon/internal/memory"
	_ "github.com/sonicboom15/daimon/internal/components/graph/memgraph"
)

// fakeMemgraphHTTP is a minimal Memgraph HTTP API fake.
type fakeMemgraphHTTP struct{}

func (f *fakeMemgraphHTTP) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var body struct {
		Query string `json:"query"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	var rows []any
	if strings.Contains(body.Query, "RETURN") {
		rows = []any{map[string]any{"n": map[string]any{"id": "x"}}}
	}
	json.NewEncoder(w).Encode(map[string]any{"results": rows}) //nolint:errcheck
}

func newStore(t *testing.T) memory.GraphStore {
	t.Helper()
	fake := &fakeMemgraphHTTP{}
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)

	gs, err := memory.NewGraph("memgraph", memory.StoreConfig{
		Metadata: map[string]string{
			"protocol": "http",
			"http_url": srv.URL,
		},
	})
	if err != nil {
		t.Fatalf("memory.NewGraph: %v", err)
	}
	return gs
}

func TestMemgraph_AddNode(t *testing.T) {
	gs := newStore(t)
	id, err := gs.AddNode(context.Background(), "n1", []string{"Node"}, map[string]any{"val": 1})
	if err != nil {
		t.Fatalf("AddNode: %v", err)
	}
	if id != "n1" {
		t.Errorf("id = %q, want n1", id)
	}
}

func TestMemgraph_AddEdge(t *testing.T) {
	gs := newStore(t)
	if err := gs.AddEdge(context.Background(), "a", "b", "LINKED_TO", nil); err != nil {
		t.Fatalf("AddEdge: %v", err)
	}
}

func TestMemgraph_Cypher(t *testing.T) {
	gs := newStore(t)
	rows, err := gs.Cypher(context.Background(), "MATCH (n) RETURN n", nil)
	if err != nil {
		t.Fatalf("Cypher: %v", err)
	}
	_ = rows
}

func TestMemgraph_Delete(t *testing.T) {
	gs := newStore(t)
	if err := gs.Delete(context.Background(), "n1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

func TestMemgraph_AddEdge_InvalidRelType(t *testing.T) {
	gs := newStore(t)
	err := gs.AddEdge(context.Background(), "a", "b", "BAD TYPE", nil)
	if err == nil {
		t.Fatal("expected error for invalid relationship type")
	}
}
