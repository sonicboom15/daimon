// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package chroma_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sonicboom15/daimon/internal/memory"
	_ "github.com/sonicboom15/daimon/internal/components/vector/chroma"
)

// chromaServer is a minimal fake Chroma HTTP API.
type chromaServer struct {
	docs map[string]struct {
		content  string
		metadata map[string]string
	}
}

func newChromaServer() *chromaServer {
	return &chromaServer{docs: map[string]struct {
		content  string
		metadata map[string]string
	}{}}
}

func (c *chromaServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch {
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/collections"):
		json.NewEncoder(w).Encode(map[string]any{"id": "col-id", "name": "daimon"}) //nolint:errcheck

	case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/collections/"):
		// collection already exists
		json.NewEncoder(w).Encode(map[string]any{"id": "col-id", "name": "daimon"}) //nolint:errcheck

	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/upsert"):
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		ids := body["ids"].([]any)
		docs := body["documents"].([]any)
		metas, _ := body["metadatas"].([]any)
		for i, id := range ids {
			meta := map[string]string{}
			if metas != nil && i < len(metas) {
				if m, ok := metas[i].(map[string]any); ok {
					for k, v := range m {
						if s, ok := v.(string); ok {
							meta[k] = s
						}
					}
				}
			}
			c.docs[id.(string)] = struct {
				content  string
				metadata map[string]string
			}{content: docs[i].(string), metadata: meta}
		}
		w.WriteHeader(http.StatusOK)

	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/query"):
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		nResults := int(body["n_results"].(float64))
		var ids []string
		var docs []string
		var distances []float64
		var metas []map[string]any
		for id, d := range c.docs {
			ids = append(ids, id)
			docs = append(docs, d.content)
			distances = append(distances, 0.1)
			m := map[string]any{}
			for k, v := range d.metadata {
				m[k] = v
			}
			metas = append(metas, m)
			if len(ids) >= nResults {
				break
			}
		}
		resp := map[string]any{
			"ids":       []any{ids},
			"documents": []any{docs},
			"distances": []any{distances},
			"metadatas": []any{metas},
		}
		json.NewEncoder(w).Encode(resp) //nolint:errcheck

	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/delete"):
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if idsRaw, ok := body["ids"].([]any); ok {
			for _, id := range idsRaw {
				delete(c.docs, id.(string))
			}
		}
		w.WriteHeader(http.StatusOK)

	default:
		http.NotFound(w, r)
	}
}

func newStore(t *testing.T) (memory.MemoryStore, *chromaServer) {
	t.Helper()
	cs := newChromaServer()
	srv := httptest.NewServer(cs)
	t.Cleanup(srv.Close)

	ms, err := memory.New("chroma", memory.StoreConfig{
		Metadata: map[string]string{
			"base_url":          srv.URL,
			"collection":        "daimon",
			"create_if_missing": "true",
		},
	})
	if err != nil {
		t.Fatalf("memory.New: %v", err)
	}
	return ms, cs
}

func TestChroma_UpsertAndQuery(t *testing.T) {
	ms, _ := newStore(t)

	id, err := ms.Upsert(context.Background(), "doc1", "The Eiffel Tower is in Paris", nil)
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if id != "doc1" {
		t.Errorf("id = %q, want doc1", id)
	}

	results, err := ms.Query(context.Background(), "Paris", 5)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected results, got 0")
	}
	if results[0].ID != "doc1" {
		t.Errorf("top result id = %q, want doc1", results[0].ID)
	}
}

func TestChroma_Delete(t *testing.T) {
	ms, cs := newStore(t)
	_, _ = ms.Upsert(context.Background(), "doc1", "some content", nil)

	if err := ms.Delete(context.Background(), "doc1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, ok := cs.docs["doc1"]; ok {
		t.Error("document still present after delete")
	}
}

func TestChroma_MetadataRoundtrip(t *testing.T) {
	ms, _ := newStore(t)
	meta := map[string]string{"source": "wiki"}
	_, _ = ms.Upsert(context.Background(), "doc1", "mountains content", meta)

	results, _ := ms.Query(context.Background(), "mountains", 1)
	if len(results) == 0 {
		t.Fatal("no results")
	}
	if results[0].Metadata["source"] != "wiki" {
		t.Errorf("metadata source = %q, want wiki", results[0].Metadata["source"])
	}
}
