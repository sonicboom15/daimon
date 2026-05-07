// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

// Package chroma provides a MemoryStore backed by the Chroma vector database HTTP API.
// Chroma handles embeddings server-side; no local embedding component is needed.
// Register type: "chroma".
//
// Metadata keys:
//
//	base_url          — default http://localhost:8000
//	collection        — Chroma collection name, default daimon
//	create_if_missing — "true" to auto-create the collection on New()
package chroma

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/sonicboom15/daimon/internal/memory"
)

func init() {
	memory.Register("chroma", func(cfg memory.StoreConfig) (memory.MemoryStore, error) {
		return New(cfg)
	})
}

// Store calls the Chroma HTTP API v1.
type Store struct {
	baseURL    string
	collection string
	colID      string // resolved collection ID
	client     *http.Client
}

// New creates a Store and optionally creates the collection.
func New(cfg memory.StoreConfig) (*Store, error) {
	meta := cfg.Metadata

	baseURL := strings.TrimRight(meta["base_url"], "/")
	if baseURL == "" {
		baseURL = "http://localhost:8000"
	}
	collection := meta["collection"]
	if collection == "" {
		collection = "daimon"
	}

	s := &Store{
		baseURL:    baseURL,
		collection: collection,
		client:     &http.Client{},
	}

	if meta["create_if_missing"] == "true" {
		if err := s.ensureCollection(context.Background()); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (s *Store) collectionURL() string {
	return fmt.Sprintf("%s/api/v1/collections/%s", s.baseURL, s.collection)
}

func (s *Store) ensureCollection(ctx context.Context) error {
	// Try GET first.
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, s.collectionURL(), nil)
	resp, err := s.client.Do(req)
	if err == nil && resp.StatusCode == http.StatusOK {
		resp.Body.Close()
		return nil
	}
	if resp != nil {
		resp.Body.Close()
	}

	// Create it.
	body, _ := json.Marshal(map[string]string{"name": s.collection})
	req, _ = http.NewRequestWithContext(ctx, http.MethodPost,
		s.baseURL+"/api/v1/collections", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err = s.client.Do(req)
	if err != nil {
		return fmt.Errorf("chroma: create collection: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("chroma: create collection status %d: %s", resp.StatusCode, b)
	}
	return nil
}

func (s *Store) Upsert(ctx context.Context, id, content string, metadata map[string]string) (string, error) {
	if id == "" {
		id = uuid.NewString()
	}

	var meta any = nil
	if len(metadata) > 0 {
		m := make(map[string]any, len(metadata))
		for k, v := range metadata {
			m[k] = v
		}
		meta = []any{m}
	} else {
		meta = []any{nil}
	}

	body, _ := json.Marshal(map[string]any{
		"ids":       []string{id},
		"documents": []string{content},
		"metadatas": meta,
	})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		s.collectionURL()+"/upsert", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("chroma: upsert: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("chroma: upsert status %d: %s", resp.StatusCode, b)
	}
	return id, nil
}

func (s *Store) Query(ctx context.Context, query string, topK int) ([]memory.Result, error) {
	if topK <= 0 {
		topK = 5
	}
	body, _ := json.Marshal(map[string]any{
		"query_texts": []string{query},
		"n_results":   topK,
	})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		s.collectionURL()+"/query", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("chroma: query: %w", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("chroma: query status %d: %s", resp.StatusCode, b)
	}

	var result struct {
		IDs       [][]string       `json:"ids"`
		Documents [][]string       `json:"documents"`
		Distances [][]float64      `json:"distances"`
		Metadatas [][]map[string]any `json:"metadatas"`
	}
	if err := json.Unmarshal(b, &result); err != nil {
		return nil, fmt.Errorf("chroma: decode response: %w", err)
	}
	if len(result.IDs) == 0 {
		return nil, nil
	}

	ids := result.IDs[0]
	docs := result.Documents[0]
	dists := result.Distances[0]
	metas := result.Metadatas[0]

	out := make([]memory.Result, len(ids))
	for i := range ids {
		score := 0.0
		if i < len(dists) {
			score = 1.0 - dists[i]
		}
		meta := map[string]string{}
		if i < len(metas) && metas[i] != nil {
			for k, v := range metas[i] {
				if sv, ok := v.(string); ok {
					meta[k] = sv
				}
			}
		}
		out[i] = memory.Result{
			ID:       ids[i],
			Content:  docs[i],
			Metadata: meta,
			Score:    score,
		}
	}
	return out, nil
}

func (s *Store) Delete(ctx context.Context, id string) error {
	body, _ := json.Marshal(map[string]any{"ids": []string{id}})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		s.collectionURL()+"/delete", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("chroma: delete: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("chroma: delete status %d: %s", resp.StatusCode, b)
	}
	return nil
}
