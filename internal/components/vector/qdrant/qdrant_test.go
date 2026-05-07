// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package qdrant_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sonicboom15/daimon/internal/memory"
	_ "github.com/sonicboom15/daimon/internal/components/vector/qdrant"
)

// fakeEmbedServer returns a fixed-size embedding for any input.
func fakeEmbedServer(dims int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Input []string `json:"input"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		type datum struct {
			Object    string    `json:"object"`
			Index     int       `json:"index"`
			Embedding []float32 `json:"embedding"`
		}
		var data []datum
		for i := range req.Input {
			vec := make([]float32, dims)
			for j := range vec {
				vec[j] = float32(i+j) * 0.01
			}
			data = append(data, datum{Object: "embedding", Index: i, Embedding: vec})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"object": "list", "data": data})
	}))
}

// fakeQdrantServer is a minimal Qdrant REST API fake.
type fakeQdrantServer struct {
	points map[string]map[string]any
}

func newFakeQdrant() *fakeQdrantServer {
	return &fakeQdrantServer{points: map[string]map[string]any{}}
}

func (q *fakeQdrantServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch {
	case r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/points"):
		// upsert points — must be checked before the /collections/ case
		var body struct {
			Points []struct {
				ID      string         `json:"id"`
				Payload map[string]any `json:"payload"`
				Vector  []float32      `json:"vector"`
			} `json:"points"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		for _, p := range body.Points {
			q.points[p.ID] = p.Payload
		}
		json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{"operation_id": 1}, "status": "ok"}) //nolint:errcheck

	case r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/collections/"):
		// create collection
		json.NewEncoder(w).Encode(map[string]any{"result": true, "status": "ok"}) //nolint:errcheck

	case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/points/search"):
		var hits []map[string]any
		for id, payload := range q.points {
			hits = append(hits, map[string]any{"id": id, "score": 0.9, "payload": payload})
		}
		json.NewEncoder(w).Encode(map[string]any{"result": hits, "status": "ok"}) //nolint:errcheck

	case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/points/delete"):
		var body struct {
			Points []string `json:"points"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		for _, id := range body.Points {
			delete(q.points, id)
		}
		json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{"operation_id": 2}, "status": "ok"}) //nolint:errcheck

	default:
		http.NotFound(w, r)
	}
}

func newStores(t *testing.T) memory.MemoryStore {
	t.Helper()
	embedSrv := fakeEmbedServer(4)
	t.Cleanup(embedSrv.Close)

	qdrantFake := newFakeQdrant()
	qdrantSrv := httptest.NewServer(qdrantFake)
	t.Cleanup(qdrantSrv.Close)

	ms, err := memory.New("qdrant", memory.StoreConfig{
		Metadata: map[string]string{
			"base_url":          qdrantSrv.URL,
			"collection":        "test",
			"create_if_missing": "true",
			"embedding_url":     embedSrv.URL,
			"dimensions":        "4",
		},
	})
	if err != nil {
		t.Fatalf("memory.New: %v", err)
	}
	return ms
}

func TestQdrant_UpsertAndQuery(t *testing.T) {
	ms := newStores(t)

	id, err := ms.Upsert(context.Background(), "p1", "hello world", nil)
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if id != "p1" {
		t.Errorf("id = %q, want p1", id)
	}

	results, err := ms.Query(context.Background(), "hello", 5)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected results, got 0")
	}
}

func TestQdrant_Delete(t *testing.T) {
	ms := newStores(t)
	_, _ = ms.Upsert(context.Background(), "p1", "content", nil)

	if err := ms.Delete(context.Background(), "p1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

func TestQdrant_AssignsIDWhenEmpty(t *testing.T) {
	ms := newStores(t)
	id, err := ms.Upsert(context.Background(), "", "no id content", nil)
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if id == "" {
		t.Error("expected non-empty assigned id")
	}
}
