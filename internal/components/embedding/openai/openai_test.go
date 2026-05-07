// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package openai_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sonicboom15/daimon/internal/embedding"
	_ "github.com/sonicboom15/daimon/internal/components/embedding/openai"
)

// embeddingResponse mirrors the OpenAI /v1/embeddings response shape.
type embeddingResponse struct {
	Object string `json:"object"`
	Data   []struct {
		Object    string    `json:"object"`
		Index     int       `json:"index"`
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
	Model string `json:"model"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

func makeServer(t *testing.T, dims int, inputs int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/embeddings" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		resp := embeddingResponse{Object: "list", Model: "text-embedding-3-small"}
		for i := range inputs {
			vec := make([]float32, dims)
			for j := range vec {
				vec[j] = float32(i*dims+j) * 0.001
			}
			resp.Data = append(resp.Data, struct {
				Object    string    `json:"object"`
				Index     int       `json:"index"`
				Embedding []float32 `json:"embedding"`
			}{Object: "embedding", Index: i, Embedding: vec})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

func TestEmbed_ReturnsVectors(t *testing.T) {
	srv := makeServer(t, 4, 2)
	defer srv.Close()

	emb, err := embedding.New("embedding/openai", embedding.EmbedConfig{
		Metadata: map[string]string{
			"base_url":   srv.URL,
			"api_key":    "test-key",
			"model":      "text-embedding-3-small",
			"dimensions": "4",
		},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	vecs, err := emb.Embed(context.Background(), []string{"hello", "world"})
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if len(vecs) != 2 {
		t.Fatalf("got %d vectors, want 2", len(vecs))
	}
	for i, v := range vecs {
		if len(v) != 4 {
			t.Errorf("vecs[%d] has %d dims, want 4", i, len(v))
		}
	}
}

func TestDimensions(t *testing.T) {
	emb, err := embedding.New("embedding/openai", embedding.EmbedConfig{
		Metadata: map[string]string{
			"base_url":   "http://localhost:1",
			"api_key":    "x",
			"dimensions": "1536",
		},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if emb.Dimensions() != 1536 {
		t.Errorf("Dimensions() = %d, want 1536", emb.Dimensions())
	}
}

func TestEmbed_EmptyInput(t *testing.T) {
	srv := makeServer(t, 4, 0)
	defer srv.Close()

	emb, err := embedding.New("embedding/openai", embedding.EmbedConfig{
		Metadata: map[string]string{
			"base_url": srv.URL,
			"api_key":  "x",
		},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	vecs, err := emb.Embed(context.Background(), []string{})
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if len(vecs) != 0 {
		t.Errorf("expected 0 vectors for empty input, got %d", len(vecs))
	}
}

func TestEmbed_ServerError(t *testing.T) {
	errSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer errSrv.Close()

	emb, err := embedding.New("embedding/openai", embedding.EmbedConfig{
		Metadata: map[string]string{
			"base_url": errSrv.URL,
			"api_key":  "x",
		},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = emb.Embed(context.Background(), []string{"text"})
	if err == nil {
		t.Fatal("expected error on server 500, got nil")
	}
}
