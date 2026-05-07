// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

// Package redis provides a MemoryStore backed by Redis Stack (RediSearch + JSON modules).
// Vectors are stored in Redis hashes with HNSW indexing via FT.CREATE / FT.SEARCH.
// Register type: "redis".
//
// Metadata keys:
//
//	addr          — Redis address, default localhost:6379
//	password      — optional
//	db            — Redis DB index, default 0
//	index         — FT index name, default daimon
//	embedding_url — OpenAI-compatible endpoint for embedding generation
//	dimensions    — embedding dimensions, default 1536
//
// Note: requires Redis Stack (or redis/redis-stack Docker image) for FT commands.
package redis

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"

	"github.com/sonicboom15/daimon/internal/memory"
)

func init() {
	memory.Register("redis", func(cfg memory.StoreConfig) (memory.MemoryStore, error) {
		return New(cfg)
	})
}

// Store wraps Redis Stack for vector similarity search.
type Store struct {
	client     *goredis.Client
	index      string
	embedURL   string
	dimensions int
	httpClient *http.Client
}

// New creates a Store and ensures the FT index exists.
func New(cfg memory.StoreConfig) (*Store, error) {
	meta := cfg.Metadata

	addr := meta["addr"]
	if addr == "" {
		addr = "localhost:6379"
	}

	db := 0
	if s := meta["db"]; s != "" {
		var err error
		db, err = strconv.Atoi(s)
		if err != nil {
			return nil, fmt.Errorf("redis vector: invalid db %q: %w", s, err)
		}
	}

	index := meta["index"]
	if index == "" {
		index = "daimon"
	}

	dims := 1536
	if d := meta["dimensions"]; d != "" {
		var err error
		dims, err = strconv.Atoi(d)
		if err != nil {
			return nil, fmt.Errorf("redis vector: invalid dimensions %q: %w", d, err)
		}
	}

	client := goredis.NewClient(&goredis.Options{
		Addr:     addr,
		Password: meta["password"],
		DB:       db,
	})

	s := &Store{
		client:     client,
		index:      index,
		embedURL:   strings.TrimRight(meta["embedding_url"], "/"),
		dimensions: dims,
		httpClient: &http.Client{},
	}

	// Best-effort index creation; non-fatal if Redis Stack isn't available.
	_ = s.ensureIndex(context.Background())
	return s, nil
}

func (s *Store) ensureIndex(ctx context.Context) error {
	err := s.client.Do(ctx,
		"FT.CREATE", s.index,
		"ON", "HASH",
		"PREFIX", "1", s.index+":",
		"SCHEMA",
		"content", "TEXT",
		"metadata", "TEXT",
		"embedding", "VECTOR", "HNSW", "6",
		"TYPE", "FLOAT32",
		"DIM", strconv.Itoa(s.dimensions),
		"DISTANCE_METRIC", "COSINE",
	).Err()
	if err != nil && strings.Contains(err.Error(), "Index already exists") {
		return nil
	}
	return err
}

func (s *Store) embed(ctx context.Context, text string) ([]float32, error) {
	if s.embedURL == "" {
		// Deterministic hash vector for dev/test.
		vec := make([]float32, s.dimensions)
		for i, c := range text {
			vec[i%s.dimensions] += float32(c)
		}
		return vec, nil
	}
	body, _ := json.Marshal(map[string]any{"model": "text-embedding", "input": []string{text}})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, s.embedURL+"/v1/embeddings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("redis vector: embed: %w", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("redis vector: embed status %d: %s", resp.StatusCode, b)
	}
	var result struct {
		Data []struct {
			Index     int       `json:"index"`
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.Unmarshal(b, &result); err != nil {
		return nil, fmt.Errorf("redis vector: embed decode: %w", err)
	}
	if len(result.Data) == 0 {
		return nil, fmt.Errorf("redis vector: embed returned no data")
	}
	return result.Data[0].Embedding, nil
}

// encodeFloat32s encodes a []float32 as little-endian IEEE 754 bytes.
// This is the format Redis Stack expects for VECTOR fields.
func encodeFloat32s(vecs []float32) []byte {
	buf := make([]byte, len(vecs)*4)
	for i, f := range vecs {
		bits := math.Float32bits(f)
		buf[i*4+0] = byte(bits)
		buf[i*4+1] = byte(bits >> 8)
		buf[i*4+2] = byte(bits >> 16)
		buf[i*4+3] = byte(bits >> 24)
	}
	return buf
}

func (s *Store) Upsert(ctx context.Context, id, content string, metadata map[string]string) (string, error) {
	if id == "" {
		id = uuid.NewString()
	}
	vec, err := s.embed(ctx, content)
	if err != nil {
		return "", err
	}
	metaJSON, _ := json.Marshal(metadata)

	err = s.client.HSet(ctx, s.index+":"+id,
		"content", content,
		"metadata", string(metaJSON),
		"embedding", encodeFloat32s(vec),
	).Err()
	if err != nil {
		return "", fmt.Errorf("redis vector: HSET: %w", err)
	}
	return id, nil
}

func (s *Store) Query(ctx context.Context, query string, topK int) ([]memory.Result, error) {
	if topK <= 0 {
		topK = 5
	}
	vec, err := s.embed(ctx, query)
	if err != nil {
		return nil, err
	}
	vecBytes := encodeFloat32s(vec)

	// FT.SEARCH with KNN vector search.
	raw, err := s.client.Do(ctx,
		"FT.SEARCH", s.index,
		"*=>[KNN "+strconv.Itoa(topK)+" @embedding $vec AS score]",
		"PARAMS", "2", "vec", vecBytes,
		"SORTBY", "score",
		"LIMIT", "0", strconv.Itoa(topK),
		"RETURN", "3", "content", "metadata", "score",
	).Result()
	if err != nil {
		return nil, fmt.Errorf("redis vector: FT.SEARCH: %w", err)
	}

	return parseFTSearch(raw), nil
}

// parseFTSearch decodes the raw FT.SEARCH reply into Results.
// FT.SEARCH returns: [count, key1, [field1, val1, ...], key2, ...]
func parseFTSearch(raw any) []memory.Result {
	arr, ok := raw.([]any)
	if !ok || len(arr) < 1 {
		return nil
	}
	var results []memory.Result
	// arr[0] is count, then pairs of (key, []field/value)
	for i := 1; i < len(arr); i += 2 {
		if i+1 >= len(arr) {
			break
		}
		fields, ok := arr[i+1].([]any)
		if !ok {
			continue
		}
		r := memory.Result{}
		if key, ok := arr[i].(string); ok {
			// key is "index:id"
			parts := strings.SplitN(key, ":", 2)
			if len(parts) == 2 {
				r.ID = parts[1]
			} else {
				r.ID = key
			}
		}
		for j := 0; j+1 < len(fields); j += 2 {
			k, _ := fields[j].(string)
			v, _ := fields[j+1].(string)
			switch k {
			case "content":
				r.Content = v
			case "score":
				score, _ := strconv.ParseFloat(v, 64)
				r.Score = score
			case "metadata":
				m := map[string]string{}
				_ = json.Unmarshal([]byte(v), &m)
				r.Metadata = m
			}
		}
		results = append(results, r)
	}
	return results
}

func (s *Store) Delete(ctx context.Context, id string) error {
	if err := s.client.Del(ctx, s.index+":"+id).Err(); err != nil {
		return fmt.Errorf("redis vector: DEL: %w", err)
	}
	return nil
}
