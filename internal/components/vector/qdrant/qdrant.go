// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

// Package qdrant provides a MemoryStore backed by the Qdrant REST API.
// Register type: "qdrant".
//
// Metadata keys:
//
//	base_url          — default http://localhost:6333
//	collection        — Qdrant collection name, default daimon
//	api_key           — optional (Qdrant Cloud)
//	create_if_missing — "true" to auto-create the collection on New()
//	embedding_url     — OpenAI-compatible embeddings endpoint URL (required for semantic search)
//	dimensions        — embedding dimensions, default 1536
package qdrant

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/sonicboom15/daimon/internal/memory"
)

func init() {
	memory.Register("qdrant", func(cfg memory.StoreConfig) (memory.MemoryStore, error) {
		return New(cfg)
	})
}

// Store wraps the Qdrant REST API.
type Store struct {
	baseURL    string
	collection string
	apiKey     string
	embedURL   string
	dimensions int
	client     *http.Client
}

// New creates a Store from the provided config.
func New(cfg memory.StoreConfig) (*Store, error) {
	meta := cfg.Metadata

	baseURL := strings.TrimRight(meta["base_url"], "/")
	if baseURL == "" {
		baseURL = "http://localhost:6333"
	}
	collection := meta["collection"]
	if collection == "" {
		collection = "daimon"
	}
	dims := 1536
	if d := meta["dimensions"]; d != "" {
		var err error
		dims, err = strconv.Atoi(d)
		if err != nil {
			return nil, fmt.Errorf("qdrant: invalid dimensions %q: %w", d, err)
		}
	}

	s := &Store{
		baseURL:    baseURL,
		collection: collection,
		apiKey:     meta["api_key"],
		embedURL:   strings.TrimRight(meta["embedding_url"], "/"),
		dimensions: dims,
		client:     &http.Client{},
	}

	if meta["create_if_missing"] == "true" {
		if err := s.ensureCollection(context.Background()); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (s *Store) do(ctx context.Context, method, path string, body any) ([]byte, int, error) {
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, _ := http.NewRequestWithContext(ctx, method, s.baseURL+path, r)
	req.Header.Set("Content-Type", "application/json")
	if s.apiKey != "" {
		req.Header.Set("api-key", s.apiKey)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return b, resp.StatusCode, nil
}

func (s *Store) ensureCollection(ctx context.Context) error {
	// Check whether the collection already exists before trying to create it.
	// Qdrant returns 400 on PUT if the collection is present, which would
	// cause New() to fail even when the store is fully operational.
	_, status, err := s.do(ctx, http.MethodGet, "/collections/"+s.collection, nil)
	if err != nil {
		return fmt.Errorf("qdrant: check collection: %w", err)
	}
	if status == http.StatusOK {
		return nil // already exists
	}
	if status != http.StatusNotFound {
		return fmt.Errorf("qdrant: check collection status %d", status)
	}

	_, status, err = s.do(ctx, http.MethodPut,
		"/collections/"+s.collection,
		map[string]any{
			"vectors": map[string]any{
				"size":     s.dimensions,
				"distance": "Cosine",
			},
		})
	if err != nil {
		return fmt.Errorf("qdrant: create collection: %w", err)
	}
	if status >= 300 {
		return fmt.Errorf("qdrant: create collection status %d", status)
	}
	return nil
}

func (s *Store) embed(ctx context.Context, texts []string) ([][]float32, error) {
	if s.embedURL == "" {
		// Deterministic hash vector for dev/test when no embedder is configured.
		vecs := make([][]float32, len(texts))
		for i, t := range texts {
			v := make([]float32, s.dimensions)
			for j, c := range t {
				v[j%s.dimensions] += float32(c)
			}
			vecs[i] = v
		}
		return vecs, nil
	}

	body, _ := json.Marshal(map[string]any{"model": "text-embedding", "input": texts})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, s.embedURL+"/v1/embeddings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("qdrant: embed: %w", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("qdrant: embed status %d: %s", resp.StatusCode, b)
	}
	var result struct {
		Data []struct {
			Index     int       `json:"index"`
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.Unmarshal(b, &result); err != nil {
		return nil, fmt.Errorf("qdrant: embed decode: %w", err)
	}
	vecs := make([][]float32, len(texts))
	for _, d := range result.Data {
		if d.Index < len(vecs) {
			vecs[d.Index] = d.Embedding
		}
	}
	return vecs, nil
}

func (s *Store) Upsert(ctx context.Context, id, content string, metadata map[string]string) (string, error) {
	if id == "" {
		id = uuid.NewString()
	}
	vecs, err := s.embed(ctx, []string{content})
	if err != nil {
		return "", err
	}

	payload := map[string]any{"content": content}
	for k, v := range metadata {
		payload[k] = v
	}

	b, status, err := s.do(ctx, http.MethodPut,
		"/collections/"+s.collection+"/points",
		map[string]any{
			"points": []map[string]any{
				{"id": id, "vector": vecs[0], "payload": payload},
			},
		})
	if err != nil {
		return "", fmt.Errorf("qdrant: upsert: %w", err)
	}
	if status >= 300 {
		return "", fmt.Errorf("qdrant: upsert status %d: %s", status, b)
	}
	return id, nil
}

func (s *Store) Query(ctx context.Context, query string, topK int) ([]memory.Result, error) {
	if topK <= 0 {
		topK = 5
	}
	vecs, err := s.embed(ctx, []string{query})
	if err != nil {
		return nil, err
	}

	b, status, err := s.do(ctx, http.MethodPost,
		"/collections/"+s.collection+"/points/search",
		map[string]any{
			"vector":       vecs[0],
			"limit":        topK,
			"with_payload": true,
		})
	if err != nil {
		return nil, fmt.Errorf("qdrant: search: %w", err)
	}
	if status >= 300 {
		return nil, fmt.Errorf("qdrant: search status %d: %s", status, b)
	}

	var resp struct {
		Result []struct {
			ID      string         `json:"id"`
			Score   float64        `json:"score"`
			Payload map[string]any `json:"payload"`
		} `json:"result"`
	}
	if err := json.Unmarshal(b, &resp); err != nil {
		return nil, fmt.Errorf("qdrant: search decode: %w", err)
	}

	results := make([]memory.Result, len(resp.Result))
	for i, r := range resp.Result {
		content, _ := r.Payload["content"].(string)
		meta := map[string]string{}
		for k, v := range r.Payload {
			if k == "content" {
				continue
			}
			if sv, ok := v.(string); ok {
				meta[k] = sv
			}
		}
		results[i] = memory.Result{ID: r.ID, Content: content, Score: r.Score, Metadata: meta}
	}
	return results, nil
}

func (s *Store) Delete(ctx context.Context, id string) error {
	b, status, err := s.do(ctx, http.MethodPost,
		"/collections/"+s.collection+"/points/delete",
		map[string]any{"points": []string{id}})
	if err != nil {
		return fmt.Errorf("qdrant: delete: %w", err)
	}
	if status >= 300 {
		return fmt.Errorf("qdrant: delete status %d: %s", status, b)
	}
	return nil
}
