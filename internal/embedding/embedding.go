// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

// Package embedding defines the Embedder interface and factory registry used
// by vector store components that need to generate dense vector representations.
package embedding

import "context"

// EmbedConfig is handed to every EmbedFactory at construction time.
type EmbedConfig struct {
	Metadata map[string]string
}

// Embedder generates dense vector embeddings from text.
type Embedder interface {
	// Embed returns one float32 vector per input text, in input order.
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	// Dimensions returns the fixed embedding size (e.g. 1536).
	Dimensions() int
}

// EmbedFactory creates an Embedder from an EmbedConfig.
type EmbedFactory func(cfg EmbedConfig) (Embedder, error)
