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
	_ "github.com/sonicboom15/daimon/internal/components/vector/inmemory"
)

func newTestServerWithStore(t *testing.T, storeName string) *testServer {
	t.Helper()
	ms, err := memory.New("inmemory", memory.StoreConfig{})
	if err != nil {
		t.Fatalf("memory.New: %v", err)
	}
	srv := newTestServer(&fakeConversation{calls: [][]conversation.Chunk{{}}})
	srv.stores[storeName] = ms
	return srv
}

func TestMemoryUpsertWithID(t *testing.T) {
	srv := newTestServerWithStore(t, "docs")

	body := `{"content":"Paris is the capital of France","metadata":{"lang":"en"}}`
	req := httptest.NewRequest("PUT", "/v1/memory/docs/doc1", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", w.Code, w.Body.String())
	}
	var resp map[string]string
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp["id"] != "doc1" {
		t.Errorf("id = %q, want doc1", resp["id"])
	}
}

func TestMemoryUpsertServerAssignsID(t *testing.T) {
	srv := newTestServerWithStore(t, "docs")

	body := `{"content":"some content"}`
	req := httptest.NewRequest("POST", "/v1/memory/docs", bytes.NewBufferString(body))
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

func TestMemoryQuery(t *testing.T) {
	srv := newTestServerWithStore(t, "docs")

	// Seed a document.
	srv.stores["docs"].Upsert(context.Background(), "doc1", "Eiffel Tower Paris France", nil) //nolint:errcheck

	body := `{"query":"Paris tower","top_k":5}`
	req := httptest.NewRequest("POST", "/v1/memory/docs/query", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", w.Code, w.Body.String())
	}
	var resp struct {
		Results []memory.Result `json:"results"`
	}
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Results) == 0 {
		t.Error("expected at least one result")
	}
}

func TestMemoryDelete(t *testing.T) {
	srv := newTestServerWithStore(t, "docs")
	srv.stores["docs"].Upsert(context.Background(), "doc1", "content", nil) //nolint:errcheck

	req := httptest.NewRequest("DELETE", "/v1/memory/docs/doc1", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", w.Code)
	}
}

func TestMemoryUnknownStore(t *testing.T) {
	srv := newTestServer(&fakeConversation{calls: [][]conversation.Chunk{{}}})

	for _, path := range []string{
		"/v1/memory/nostore/doc1",
		"/v1/memory/nostore/query",
		"/v1/memory/nostore",
	} {
		method := http.MethodPost
		if path == "/v1/memory/nostore/doc1" {
			method = http.MethodDelete
		}
		req := httptest.NewRequest(method, path, bytes.NewBufferString("{}"))
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Errorf("path %s: status = %d, want 404", path, w.Code)
		}
	}
}

func TestMemoryUpsert_BadBody(t *testing.T) {
	srv := newTestServerWithStore(t, "docs")

	req := httptest.NewRequest("POST", "/v1/memory/docs", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}
