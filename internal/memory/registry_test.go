// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package memory_test

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/sonicboom15/daimon/internal/memory"
)

// fakeStore is a minimal MemoryStore for testing.
type fakeStore struct{}

func (f *fakeStore) Upsert(_ context.Context, id, _ string, _ map[string]string) (string, error) {
	if id == "" {
		return "generated-id", nil
	}
	return id, nil
}

func (f *fakeStore) Query(_ context.Context, _ string, topK int) ([]memory.Result, error) {
	return make([]memory.Result, topK), nil
}

func (f *fakeStore) Delete(_ context.Context, _ string) error { return nil }

// fakeGraph is a minimal GraphStore for testing.
type fakeGraph struct{}

func (f *fakeGraph) AddNode(_ context.Context, id string, _ []string, _ map[string]any) (string, error) {
	if id == "" {
		return "g-id", nil
	}
	return id, nil
}

func (f *fakeGraph) AddEdge(_ context.Context, _, _, _ string, _ map[string]any) error { return nil }

func (f *fakeGraph) Cypher(_ context.Context, _ string, _ map[string]any) ([]memory.GraphNode, error) {
	return nil, nil
}

func (f *fakeGraph) Delete(_ context.Context, _ string) error { return nil }

// ── Vector store registry ────────────────────────────────────────────────────

func TestRegisterAndNew_Store(t *testing.T) {
	const typeName = "test-store-register"
	memory.Register(typeName, func(_ memory.StoreConfig) (memory.MemoryStore, error) {
		return &fakeStore{}, nil
	})

	ms, err := memory.New(typeName, memory.StoreConfig{})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if ms == nil {
		t.Fatal("New returned nil store")
	}
}

func TestNew_UnknownType_Store(t *testing.T) {
	_, err := memory.New("no-such-store-xyz", memory.StoreConfig{})
	if err == nil {
		t.Fatal("expected error for unknown type, got nil")
	}
	if !strings.Contains(err.Error(), "no-such-store-xyz") {
		t.Errorf("error %q does not mention the unknown type", err.Error())
	}
}

func TestNew_ConfigPassthrough_Store(t *testing.T) {
	const typeName = "test-store-config"
	var got memory.StoreConfig
	memory.Register(typeName, func(cfg memory.StoreConfig) (memory.MemoryStore, error) {
		got = cfg
		return &fakeStore{}, nil
	})

	want := memory.StoreConfig{Metadata: map[string]string{"collection": "docs"}}
	if _, err := memory.New(typeName, want); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Metadata["collection"] != "docs" {
		t.Errorf("metadata not passed through: got %v", got.Metadata)
	}
}

func TestRegister_Concurrent_Store(t *testing.T) {
	var wg sync.WaitGroup
	for i := range 10 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			name := "concurrent-store-" + string(rune('a'+n))
			memory.Register(name, func(_ memory.StoreConfig) (memory.MemoryStore, error) {
				return &fakeStore{}, nil
			})
		}(i)
	}
	wg.Wait()
}

// ── Graph store registry ─────────────────────────────────────────────────────

func TestRegisterAndNew_Graph(t *testing.T) {
	const typeName = "test-graph-register"
	memory.RegisterGraph(typeName, func(_ memory.StoreConfig) (memory.GraphStore, error) {
		return &fakeGraph{}, nil
	})

	gs, err := memory.NewGraph(typeName, memory.StoreConfig{})
	if err != nil {
		t.Fatalf("NewGraph returned error: %v", err)
	}
	if gs == nil {
		t.Fatal("NewGraph returned nil store")
	}
}

func TestNew_UnknownType_Graph(t *testing.T) {
	_, err := memory.NewGraph("no-such-graph-xyz", memory.StoreConfig{})
	if err == nil {
		t.Fatal("expected error for unknown type, got nil")
	}
	if !strings.Contains(err.Error(), "no-such-graph-xyz") {
		t.Errorf("error %q does not mention the unknown type", err.Error())
	}
}

func TestRegister_Concurrent_Graph(t *testing.T) {
	var wg sync.WaitGroup
	for i := range 10 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			name := "concurrent-graph-" + string(rune('a'+n))
			memory.RegisterGraph(name, func(_ memory.StoreConfig) (memory.GraphStore, error) {
				return &fakeGraph{}, nil
			})
		}(i)
	}
	wg.Wait()
}

// ── Separate registries ───────────────────────────────────────────────────────

func TestRegistriesAreIndependent(t *testing.T) {
	const name = "shared-name-xyz"
	memory.Register(name, func(_ memory.StoreConfig) (memory.MemoryStore, error) {
		return &fakeStore{}, nil
	})

	// Should NOT be retrievable from the graph registry.
	_, err := memory.NewGraph(name, memory.StoreConfig{})
	if err == nil {
		t.Errorf("graph registry returned something for a vector-only name %q", name)
	}
}
