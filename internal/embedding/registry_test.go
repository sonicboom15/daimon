// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package embedding_test

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/sonicboom15/daimon/internal/embedding"
)

// fakeEmbedder is a minimal Embedder for testing.
type fakeEmbedder struct{ dims int }

func (f *fakeEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i := range texts {
		out[i] = make([]float32, f.dims)
	}
	return out, nil
}

func (f *fakeEmbedder) Dimensions() int { return f.dims }

func TestRegisterAndNew(t *testing.T) {
	const typeName = "test-embed-register"
	embedding.Register(typeName, func(_ embedding.EmbedConfig) (embedding.Embedder, error) {
		return &fakeEmbedder{dims: 4}, nil
	})

	emb, err := embedding.New(typeName, embedding.EmbedConfig{})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if emb == nil {
		t.Fatal("New returned nil embedder")
	}
	if emb.Dimensions() != 4 {
		t.Errorf("Dimensions() = %d, want 4", emb.Dimensions())
	}
}

func TestNew_UnknownType(t *testing.T) {
	_, err := embedding.New("no-such-embedder-xyz", embedding.EmbedConfig{})
	if err == nil {
		t.Fatal("expected error for unknown type, got nil")
	}
	if !strings.Contains(err.Error(), "no-such-embedder-xyz") {
		t.Errorf("error %q does not mention the unknown type", err.Error())
	}
}

func TestNew_ConfigPassthrough(t *testing.T) {
	const typeName = "test-embed-config"
	var got embedding.EmbedConfig
	embedding.Register(typeName, func(cfg embedding.EmbedConfig) (embedding.Embedder, error) {
		got = cfg
		return &fakeEmbedder{dims: 1}, nil
	})

	want := embedding.EmbedConfig{Metadata: map[string]string{"model": "text-embedding-3-small"}}
	if _, err := embedding.New(typeName, want); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Metadata["model"] != "text-embedding-3-small" {
		t.Errorf("metadata not passed through: got %v", got.Metadata)
	}
}

func TestRegister_Concurrent(t *testing.T) {
	var wg sync.WaitGroup
	for i := range 10 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			name := "concurrent-embed-" + string(rune('a'+n))
			embedding.Register(name, func(_ embedding.EmbedConfig) (embedding.Embedder, error) {
				return &fakeEmbedder{dims: 1}, nil
			})
		}(i)
	}
	wg.Wait()
}

func TestEmbed_ReturnsVectorsPerInput(t *testing.T) {
	const typeName = "test-embed-vectors"
	embedding.Register(typeName, func(_ embedding.EmbedConfig) (embedding.Embedder, error) {
		return &fakeEmbedder{dims: 8}, nil
	})
	emb, err := embedding.New(typeName, embedding.EmbedConfig{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	texts := []string{"hello", "world", "foo"}
	vecs, err := emb.Embed(context.Background(), texts)
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if len(vecs) != 3 {
		t.Errorf("got %d vectors, want 3", len(vecs))
	}
	for i, v := range vecs {
		if len(v) != 8 {
			t.Errorf("vecs[%d] has %d dims, want 8", i, len(v))
		}
	}
}
