// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package onnx_test

import (
	"context"
	"os"
	"testing"

	"github.com/sonicboom15/daimon/internal/components/ner/onnx"
	"github.com/sonicboom15/daimon/internal/ner"
)

func TestONNXDriverValidation(t *testing.T) {
	// Missing model_path
	_, err := onnx.New(ner.Config{
		Metadata: map[string]string{},
	})
	if err == nil {
		t.Fatal("expected error for missing model_path")
	}

	// Non-existent model_path
	_, err = onnx.New(ner.Config{
		Metadata: map[string]string{
			"model_path": "/tmp/nonexistent-model.onnx",
		},
	})
	if err == nil {
		t.Fatal("expected error for nonexistent model_path")
	}

	// Valid model_path (create temp dummy file)
	tmpFile, err := os.CreateTemp("", "dummy-*.onnx")
	if err != nil {
		t.Fatalf("creating temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	model, err := onnx.New(ner.Config{
		Metadata: map[string]string{
			"model_path":        tmpFile.Name(),
			"default_labels":    "disease,symptom",
			"default_threshold": "0.6",
			"threads":           "2",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error creating model: %v", err)
	}

	res, err := model.Extract(context.Background(), ner.ExtractRequest{Text: ""})
	if err != nil {
		t.Fatalf("unexpected extract error: %v", err)
	}
	if len(res.Entities) != 0 {
		t.Fatalf("expected empty entities for empty text, got %d", len(res.Entities))
	}
}

