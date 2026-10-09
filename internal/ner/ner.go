// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

// Package ner defines the Model interface, entity extraction types, and factory
// registry for named entity recognition (NER) components.
package ner

import "context"

// Entity represents an extracted span of text with an associated label and confidence.
type Entity struct {
	Text       string  `json:"text"`
	Label      string  `json:"label"`
	Start      int     `json:"start"`
	End        int     `json:"end"`
	Confidence float64 `json:"confidence"`
}

// ExtractRequest contains the text and optional extraction filters.
type ExtractRequest struct {
	Text      string   `json:"text"`
	Labels    []string `json:"labels,omitempty"`
	Threshold float64  `json:"threshold,omitempty"`
}

// ExtractResponse contains the list of extracted entities.
type ExtractResponse struct {
	Entities []Entity `json:"entities"`
}

// Model performs named entity recognition and span extraction.
type Model interface {
	Extract(ctx context.Context, req ExtractRequest) (*ExtractResponse, error)
}

// Config is handed to every NER Factory at construction time.
type Config struct {
	Metadata map[string]string
}

// Factory creates a Model from a Config.
type Factory func(cfg Config) (Model, error)

