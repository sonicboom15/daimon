// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package neo4j_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sonicboom15/daimon/internal/memory"
	_ "github.com/sonicboom15/daimon/internal/components/graph/neo4j"
)

// fakeNeo4jHTTP is a minimal Neo4j Transactional HTTP API fake.
type fakeNeo4jHTTP struct {
	nodes map[string]map[string]any
	edges []map[string]any
}

func newFakeNeo4j() *fakeNeo4jHTTP {
	return &fakeNeo4jHTTP{nodes: map[string]map[string]any{}}
}

func (n *fakeNeo4jHTTP) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Neo4j basic auth
	user, pass, _ := r.BasicAuth()
	_ = user
	_ = pass

	var body struct {
		Statements []struct {
			Statement  string         `json:"statement"`
			Parameters map[string]any `json:"parameters"`
		} `json:"statements"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	// Return empty results for any Cypher.
	resp := map[string]any{
		"results": []any{
			map[string]any{"columns": []any{}, "data": []any{}},
		},
		"errors": []any{},
	}

	// Simulate node creation for AddNode calls.
	for _, stmt := range body.Statements {
		if strings.Contains(stmt.Statement, "MERGE") || strings.Contains(stmt.Statement, "CREATE") {
			if params := stmt.Parameters; params != nil {
				if id, ok := params["id"].(string); ok && id != "" {
					n.nodes[id] = params
				}
			}
		}
		if strings.Contains(stmt.Statement, "MATCH") && strings.Contains(stmt.Statement, "RETURN") {
			// Return a fake row for MATCH queries.
			resp["results"] = []any{
				map[string]any{
					"columns": []any{"n"},
					"data": []any{
						map[string]any{"row": []any{map[string]any{"name": "test"}}},
					},
				},
			}
		}
	}

	json.NewEncoder(w).Encode(resp) //nolint:errcheck
}

func newStore(t *testing.T) (memory.GraphStore, *fakeNeo4jHTTP) {
	t.Helper()
	fake := newFakeNeo4j()
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)

	gs, err := memory.NewGraph("neo4j", memory.StoreConfig{
		Metadata: map[string]string{
			"protocol": "http",
			"http_url": srv.URL,
			"username": "neo4j",
			"password": "test",
			"database": "neo4j",
		},
	})
	if err != nil {
		t.Fatalf("memory.NewGraph: %v", err)
	}
	return gs, fake
}

func TestNeo4j_AddNode(t *testing.T) {
	gs, _ := newStore(t)

	id, err := gs.AddNode(context.Background(), "alice", []string{"Person"}, map[string]any{"name": "Alice"})
	if err != nil {
		t.Fatalf("AddNode: %v", err)
	}
	if id != "alice" {
		t.Errorf("id = %q, want alice", id)
	}
}

func TestNeo4j_AddNodeAssignsIDWhenEmpty(t *testing.T) {
	gs, _ := newStore(t)

	id, err := gs.AddNode(context.Background(), "", []string{"Thing"}, nil)
	if err != nil {
		t.Fatalf("AddNode: %v", err)
	}
	if id == "" {
		t.Error("expected non-empty assigned id")
	}
}

func TestNeo4j_AddEdge(t *testing.T) {
	gs, _ := newStore(t)

	if err := gs.AddEdge(context.Background(), "alice", "bob", "KNOWS", nil); err != nil {
		t.Fatalf("AddEdge: %v", err)
	}
}

func TestNeo4j_Cypher(t *testing.T) {
	gs, _ := newStore(t)

	rows, err := gs.Cypher(context.Background(), "MATCH (n) RETURN n", nil)
	if err != nil {
		t.Fatalf("Cypher: %v", err)
	}
	// Fake returns 1 row.
	if len(rows) == 0 {
		t.Error("expected at least 1 row from MATCH query")
	}
}

func TestNeo4j_Delete(t *testing.T) {
	gs, _ := newStore(t)

	if err := gs.Delete(context.Background(), "alice"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

func TestNeo4j_AddEdge_InvalidRelType(t *testing.T) {
	gs, _ := newStore(t)

	// Relationship type with spaces / injection attempt must be rejected.
	err := gs.AddEdge(context.Background(), "a", "b", "KNOWS; DROP ALL", nil)
	if err == nil {
		t.Fatal("expected error for invalid relationship type, got nil")
	}
}
