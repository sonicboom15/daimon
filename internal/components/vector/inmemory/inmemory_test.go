// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package inmemory_test

import (
	"context"
	"testing"

	"github.com/sonicboom15/daimon/internal/memory"
	_ "github.com/sonicboom15/daimon/internal/components/vector/inmemory"
)

func newStore(t *testing.T) memory.MemoryStore {
	t.Helper()
	ms, err := memory.New("inmemory", memory.StoreConfig{})
	if err != nil {
		t.Fatalf("memory.New: %v", err)
	}
	return ms
}

func TestUpsertAndQuery(t *testing.T) {
	ms := newStore(t)

	id, err := ms.Upsert(context.Background(), "doc1", "The Eiffel Tower is in Paris France", nil)
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if id != "doc1" {
		t.Errorf("Upsert returned id %q, want doc1", id)
	}

	_, _ = ms.Upsert(context.Background(), "doc2", "The Statue of Liberty is in New York USA", nil)
	_, _ = ms.Upsert(context.Background(), "doc3", "The Colosseum is in Rome Italy", nil)

	results, err := ms.Query(context.Background(), "Paris tower France", 2)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one result, got 0")
	}
	if results[0].ID != "doc1" {
		t.Errorf("top result id = %q, want doc1", results[0].ID)
	}
}

func TestUpsertAssignsIDWhenEmpty(t *testing.T) {
	ms := newStore(t)
	id, err := ms.Upsert(context.Background(), "", "some content about cats", nil)
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if id == "" {
		t.Error("expected a non-empty assigned id")
	}

	results, _ := ms.Query(context.Background(), "cats", 5)
	found := false
	for _, r := range results {
		if r.ID == id {
			found = true
		}
	}
	if !found {
		t.Errorf("assigned id %q not found in query results", id)
	}
}

func TestDelete(t *testing.T) {
	ms := newStore(t)
	id, _ := ms.Upsert(context.Background(), "doc1", "content about dogs", nil)

	if err := ms.Delete(context.Background(), id); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	results, _ := ms.Query(context.Background(), "dogs", 10)
	for _, r := range results {
		if r.ID == id {
			t.Errorf("deleted document %q still appears in query results", id)
		}
	}
}

func TestDeleteIdempotent(t *testing.T) {
	ms := newStore(t)
	if err := ms.Delete(context.Background(), "nonexistent"); err != nil {
		t.Errorf("Delete of nonexistent id returned error: %v", err)
	}
}

func TestUpsertUpdatesExisting(t *testing.T) {
	ms := newStore(t)
	_, _ = ms.Upsert(context.Background(), "doc1", "original content about apples", nil)
	_, _ = ms.Upsert(context.Background(), "doc1", "updated content about oranges", nil)

	results, _ := ms.Query(context.Background(), "oranges", 5)
	if len(results) == 0 {
		t.Fatal("expected result for updated content")
	}
	if results[0].ID != "doc1" {
		t.Errorf("updated doc not at top: %v", results)
	}
	if results[0].Content != "updated content about oranges" {
		t.Errorf("content = %q, want updated", results[0].Content)
	}
}

func TestQuery_TopKRespected(t *testing.T) {
	ms := newStore(t)
	for i := range 10 {
		_ = i
		_, _ = ms.Upsert(context.Background(), "", "content about robots", nil)
	}
	results, _ := ms.Query(context.Background(), "robots", 3)
	if len(results) > 3 {
		t.Errorf("got %d results, want at most 3", len(results))
	}
}

func TestQuery_EmptyStore(t *testing.T) {
	ms := newStore(t)
	results, err := ms.Query(context.Background(), "anything", 5)
	if err != nil {
		t.Fatalf("Query on empty store: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results on empty store, got %d", len(results))
	}
}

func TestMetadataRoundtrip(t *testing.T) {
	ms := newStore(t)
	meta := map[string]string{"source": "wiki", "lang": "en"}
	_, _ = ms.Upsert(context.Background(), "doc1", "content about mountains", meta)

	results, _ := ms.Query(context.Background(), "mountains", 1)
	if len(results) == 0 {
		t.Fatal("no results")
	}
	if results[0].Metadata["source"] != "wiki" {
		t.Errorf("metadata source = %q, want wiki", results[0].Metadata["source"])
	}
}
