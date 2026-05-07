// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sonicboom15/daimon/internal/conversation"
	"github.com/sonicboom15/daimon/internal/memory"
	_ "github.com/sonicboom15/daimon/internal/components/graph/neo4j"
)

// fakeGraph is an in-process GraphStore for handler tests.
type fakeGraph struct {
	nodes map[string]map[string]any
}

func newFakeGraph() *fakeGraph {
	return &fakeGraph{nodes: map[string]map[string]any{}}
}

func (g *fakeGraph) AddNode(_ context.Context, id string, _ []string, props map[string]any) (string, error) {
	if id == "" {
		id = "assigned-id"
	}
	g.nodes[id] = props
	return id, nil
}

func (g *fakeGraph) AddEdge(_ context.Context, _, _, _ string, _ map[string]any) error { return nil }

func (g *fakeGraph) Cypher(_ context.Context, _ string, _ map[string]any) ([]memory.GraphNode, error) {
	return []memory.GraphNode{{"key": "value"}}, nil
}

func (g *fakeGraph) Delete(_ context.Context, id string) error {
	delete(g.nodes, id)
	return nil
}

func newTestServerWithGraph(t *testing.T, storeName string) (*testServer, *fakeGraph) {
	t.Helper()
	fg := newFakeGraph()
	srv := newTestServer(&fakeConversation{calls: [][]conversation.Chunk{{}}})
	srv.graphs[storeName] = fg
	return srv, fg
}

func TestGraphAddNodeWithID(t *testing.T) {
	srv, fg := newTestServerWithGraph(t, "kg")

	body := `{"labels":["Person"],"props":{"name":"Alice"}}`
	req := httptest.NewRequest("PUT", "/v1/graph/kg/nodes/alice", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", w.Code, w.Body.String())
	}
	var resp map[string]string
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp["id"] != "alice" {
		t.Errorf("id = %q, want alice", resp["id"])
	}
	if _, ok := fg.nodes["alice"]; !ok {
		t.Error("node alice not stored")
	}
}

func TestGraphAddNodeServerAssignsID(t *testing.T) {
	srv, _ := newTestServerWithGraph(t, "kg")

	body := `{"labels":["Thing"]}`
	req := httptest.NewRequest("POST", "/v1/graph/kg/nodes", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", w.Code, w.Body.String())
	}
	var resp map[string]string
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp["id"] == "" {
		t.Error("expected non-empty assigned id")
	}
}

func TestGraphAddEdge(t *testing.T) {
	srv, _ := newTestServerWithGraph(t, "kg")

	body := `{"from":"alice","to":"bob","type":"KNOWS"}`
	req := httptest.NewRequest("POST", "/v1/graph/kg/edges", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", w.Code)
	}
}

func TestGraphCypher(t *testing.T) {
	srv, _ := newTestServerWithGraph(t, "kg")

	body := `{"query":"MATCH (n) RETURN n","params":{}}`
	req := httptest.NewRequest("POST", "/v1/graph/kg/cypher", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", w.Code, w.Body.String())
	}
	var resp struct {
		Rows []map[string]any `json:"rows"`
	}
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Rows) == 0 {
		t.Error("expected rows from cypher query")
	}
}

func TestGraphDelete(t *testing.T) {
	srv, fg := newTestServerWithGraph(t, "kg")
	fg.nodes["n1"] = map[string]any{"id": "n1"}

	req := httptest.NewRequest("DELETE", "/v1/graph/kg/nodes/n1", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", w.Code)
	}
	if _, ok := fg.nodes["n1"]; ok {
		t.Error("node still present after delete")
	}
}

func TestGraphUnknownStore(t *testing.T) {
	srv := newTestServer(&fakeConversation{calls: [][]conversation.Chunk{{}}})

	req := httptest.NewRequest("POST", "/v1/graph/nostore/cypher", bytes.NewBufferString(`{"query":"MATCH (n) RETURN n"}`))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}
