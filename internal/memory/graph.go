// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package memory

import "context"

// GraphNode is a single row returned from a Cypher or graph query.
// Keys are column aliases; values are the raw data from the database.
type GraphNode map[string]any

// GraphStore is the interface for graph database backends.
// It is intentionally separate from MemoryStore — graph stores model entities
// and relationships, not documents and embeddings.
// All methods must be safe for concurrent use.
type GraphStore interface {
	// AddNode creates or updates a node identified by id.
	// Labels (e.g. "Person", "Document") are applied to the node.
	// Props are arbitrary key/value properties.
	// Returns the canonical node id.
	AddNode(ctx context.Context, id string, labels []string, props map[string]any) (string, error)
	// AddEdge creates a directed relationship from fromID to toID.
	// relType is the relationship label (e.g. "KNOWS", "AUTHORED").
	AddEdge(ctx context.Context, fromID, toID, relType string, props map[string]any) error
	// Cypher runs a raw query string with named parameters.
	// Each result row is returned as map[string]any keyed by column alias.
	Cypher(ctx context.Context, query string, params map[string]any) ([]GraphNode, error)
	// Delete removes the node with the given id and all its relationships.
	// Idempotent.
	Delete(ctx context.Context, id string) error
}

// GraphFactory creates a GraphStore from a StoreConfig.
type GraphFactory func(cfg StoreConfig) (GraphStore, error)
