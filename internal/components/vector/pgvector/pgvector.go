// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

// Package pgvector provides a MemoryStore backed by PostgreSQL with the pgvector extension.
// Register type: "pgvector".
//
// Metadata keys:
//
//	dsn           — PostgreSQL DSN (required)
//	table         — table name, default daimon_documents
//	embedding_url — OpenAI-compatible endpoint for embedding generation
//	dimensions    — embedding dimensions, default 1536
package pgvector

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
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sonicboom15/daimon/internal/memory"
)

func init() {
	memory.Register("pgvector", func(cfg memory.StoreConfig) (memory.MemoryStore, error) {
		return New(cfg)
	})
}

// Store uses PostgreSQL + pgvector for vector similarity search.
type Store struct {
	pool       *pgxpool.Pool
	table      string
	embedURL   string
	dimensions int
	httpClient *http.Client
}

// New creates a Store and initialises the table and index.
func New(cfg memory.StoreConfig) (*Store, error) {
	meta := cfg.Metadata

	dsn := meta["dsn"]
	if dsn == "" {
		return nil, fmt.Errorf("pgvector: dsn is required")
	}

	table := meta["table"]
	if table == "" {
		table = "daimon_documents"
	}

	dims := 1536
	if d := meta["dimensions"]; d != "" {
		var err error
		dims, err = strconv.Atoi(d)
		if err != nil {
			return nil, fmt.Errorf("pgvector: invalid dimensions %q: %w", d, err)
		}
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		return nil, fmt.Errorf("pgvector: connect: %w", err)
	}

	s := &Store{
		pool:       pool,
		table:      table,
		embedURL:   strings.TrimRight(meta["embedding_url"], "/"),
		dimensions: dims,
		httpClient: &http.Client{},
	}

	if err := s.migrate(context.Background()); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pgvector: migrate: %w", err)
	}
	return s, nil
}

func (s *Store) migrate(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS vector`)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id        TEXT PRIMARY KEY,
			content   TEXT NOT NULL,
			metadata  JSONB,
			embedding vector(%d)
		)`, s.table, s.dimensions))
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, fmt.Sprintf(`
		CREATE INDEX IF NOT EXISTS %s_embedding_idx
		ON %s USING ivfflat (embedding vector_cosine_ops)`, s.table, s.table))
	return err
}

func (s *Store) embed(ctx context.Context, text string) ([]float32, error) {
	if s.embedURL == "" {
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
		return nil, fmt.Errorf("pgvector: embed: %w", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("pgvector: embed status %d: %s", resp.StatusCode, b)
	}
	var result struct {
		Data []struct {
			Index     int       `json:"index"`
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.Unmarshal(b, &result); err != nil {
		return nil, fmt.Errorf("pgvector: embed decode: %w", err)
	}
	if len(result.Data) == 0 {
		return nil, fmt.Errorf("pgvector: embed returned no data")
	}
	return result.Data[0].Embedding, nil
}

// vecToString converts a float32 slice to Postgres vector literal: "[0.1,0.2,...]"
func vecToString(vecs []float32) string {
	var sb strings.Builder
	sb.WriteByte('[')
	for i, v := range vecs {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(strconv.FormatFloat(float64(v), 'f', 6, 32))
	}
	sb.WriteByte(']')
	return sb.String()
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

	_, err = s.pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s (id, content, metadata, embedding)
		VALUES ($1, $2, $3, $4::vector)
		ON CONFLICT (id) DO UPDATE
		SET content = $2, metadata = $3, embedding = $4::vector`,
		s.table),
		id, content, metaJSON, vecToString(vec))
	if err != nil {
		return "", fmt.Errorf("pgvector: upsert: %w", err)
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

	rows, err := s.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, content, metadata, 1-(embedding<=>$1::vector) AS score
		FROM %s
		ORDER BY embedding <=> $1::vector
		LIMIT $2`, s.table),
		vecToString(vec), topK)
	if err != nil {
		return nil, fmt.Errorf("pgvector: query: %w", err)
	}
	defer rows.Close()

	var results []memory.Result
	for rows.Next() {
		var r memory.Result
		var metaJSON []byte
		if err := rows.Scan(&r.ID, &r.Content, &metaJSON, &r.Score); err != nil {
			return nil, fmt.Errorf("pgvector: scan: %w", err)
		}
		if len(metaJSON) > 0 {
			_ = json.Unmarshal(metaJSON, &r.Metadata)
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

func (s *Store) Delete(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, fmt.Sprintf(`DELETE FROM %s WHERE id = $1`, s.table), id)
	if err != nil {
		return fmt.Errorf("pgvector: delete: %w", err)
	}
	return nil
}
