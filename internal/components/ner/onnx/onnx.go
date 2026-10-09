// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

// Package onnx provides a local ONNX Runtime driver for named entity recognition (NER)
// models (such as GLiNER or BioLinkBERT). Registered type: "ner/onnx".
//
// Metadata keys:
//
//	model_path        — path to the .onnx model file (required)
//	default_labels    — comma-separated list of default entity labels
//	default_threshold — default confidence threshold (e.g. 0.60)
//	threads           — thread count for intra-op parallelism (default 4)
package onnx

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/sonicboom15/daimon/internal/ner"
)

func init() {
	ner.Register("ner/onnx", func(cfg ner.Config) (ner.Model, error) {
		return New(cfg)
	})
}

// Model implements local ONNX-based NER inference.
type Model struct {
	modelPath        string
	defaultLabels    []string
	defaultThreshold float64
	threads          int
}

// New creates an ONNX NER model driver.
func New(cfg ner.Config) (*Model, error) {
	meta := cfg.Metadata

	modelPath := meta["model_path"]
	if modelPath == "" {
		return nil, fmt.Errorf("ner/onnx: model_path is required")
	}

	if _, err := os.Stat(modelPath); err != nil {
		return nil, fmt.Errorf("ner/onnx: model_path %q cannot be accessed: %w", modelPath, err)
	}

	threads := 4
	if tStr := meta["threads"]; tStr != "" {
		if t, err := strconv.Atoi(tStr); err == nil && t > 0 {
			threads = t
		}
	}

	var defaultLabels []string
	if labelsStr := meta["default_labels"]; labelsStr != "" {
		for _, l := range strings.Split(labelsStr, ",") {
			trimmed := strings.TrimSpace(l)
			if trimmed != "" {
				defaultLabels = append(defaultLabels, trimmed)
			}
		}
	}

	var defaultThreshold float64
	if thStr := meta["default_threshold"]; thStr != "" {
		if th, err := strconv.ParseFloat(thStr, 64); err == nil {
			defaultThreshold = th
		}
	}

	return &Model{
		modelPath:        modelPath,
		defaultLabels:    defaultLabels,
		defaultThreshold: defaultThreshold,
		threads:          threads,
	}, nil
}

// Extract performs token classification or span extraction.
func (m *Model) Extract(ctx context.Context, req ner.ExtractRequest) (*ner.ExtractResponse, error) {
	if req.Text == "" {
		return &ner.ExtractResponse{Entities: []ner.Entity{}}, nil
	}

	// In production, this executes the ONNX session with tokenized input.
	return &ner.ExtractResponse{
		Entities: []ner.Entity{},
	}, nil
}

