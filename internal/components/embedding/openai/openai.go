// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

// Package openai provides an Embedder backed by the OpenAI (or OpenAI-compatible)
// /v1/embeddings endpoint. Register type: "embedding/openai".
//
// Metadata keys:
//
//	base_url   — default https://api.openai.com
//	api_key    — falls back to OPENAI_API_KEY env var
//	model      — default text-embedding-3-small
//	dimensions — integer, default 1536
package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/sonicboom15/daimon/internal/embedding"
)

func init() {
	embedding.Register("embedding/openai", func(cfg embedding.EmbedConfig) (embedding.Embedder, error) {
		return New(cfg)
	})
}

// Embedder calls the OpenAI /v1/embeddings endpoint.
type Embedder struct {
	baseURL    string
	apiKey     string
	model      string
	dimensions int
	client     *http.Client
}

// New creates an Embedder from the provided config.
func New(cfg embedding.EmbedConfig) (*Embedder, error) {
	meta := cfg.Metadata

	baseURL := strings.TrimRight(meta["base_url"], "/")
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}

	apiKey := meta["api_key"]
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}

	model := meta["model"]
	if model == "" {
		model = "text-embedding-3-small"
	}

	dims := 1536
	if d := meta["dimensions"]; d != "" {
		var err error
		dims, err = strconv.Atoi(d)
		if err != nil {
			return nil, fmt.Errorf("embedding/openai: invalid dimensions %q: %w", d, err)
		}
	}

	return &Embedder{
		baseURL:    baseURL,
		apiKey:     apiKey,
		model:      model,
		dimensions: dims,
		client:     &http.Client{},
	}, nil
}

// Dimensions returns the configured embedding size.
func (e *Embedder) Dimensions() int { return e.dimensions }

// Embed calls the embeddings endpoint and returns one vector per input text.
func (e *Embedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	body, err := json.Marshal(map[string]any{
		"model": e.model,
		"input": texts,
	})
	if err != nil {
		return nil, fmt.Errorf("embedding/openai: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/v1/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("embedding/openai: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if e.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+e.apiKey)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedding/openai: http: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("embedding/openai: read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embedding/openai: status %d: %s", resp.StatusCode, respBytes)
	}

	var result struct {
		Data []struct {
			Index     int       `json:"index"`
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBytes, &result); err != nil {
		return nil, fmt.Errorf("embedding/openai: decode response: %w", err)
	}

	vecs := make([][]float32, len(texts))
	for _, d := range result.Data {
		if d.Index < len(vecs) {
			vecs[d.Index] = d.Embedding
		}
	}
	return vecs, nil
}
