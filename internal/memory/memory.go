// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

// Package memory defines the MemoryStore and GraphStore interfaces and their
// factory registries used by vector and graph database components.
package memory

import (
	"context"

	"github.com/sonicboom15/daimon/internal/embedding"
)

// Result is a single document returned from a vector store query.
type Result struct {
	ID       string            `json:"id"`
	Content  string            `json:"content"`
	Metadata map[string]string `json:"metadata,omitempty"`
	Score    float64           `json:"score,omitempty"`
}

// StoreConfig is handed to every StoreFactory at construction time.
type StoreConfig struct {
	Metadata map[string]string
	// Embedder is non-nil when a named embedding component has been resolved
	// for this store. Stores that do embeddings server-side (e.g. Chroma)
	// and lexical stores (inmemory) may ignore this field.
	Embedder embedding.Embedder
}

// MemoryStore is the interface for vector / document store backends.
// All methods must be safe for concurrent use.
type MemoryStore interface {
	// Upsert inserts or updates a document. An empty id causes the store to
	// assign one, which is returned. A caller-provided id is returned unchanged.
	Upsert(ctx context.Context, id, content string, metadata map[string]string) (string, error)
	// Query performs similarity search. Returns up to topK results by
	// descending score.
	Query(ctx context.Context, query string, topK int) ([]Result, error)
	// Delete removes the document with the given id. Idempotent.
	Delete(ctx context.Context, id string) error
}

// StoreFactory creates a MemoryStore from a StoreConfig.
type StoreFactory func(cfg StoreConfig) (MemoryStore, error)
