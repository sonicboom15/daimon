// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

// Package memgraph provides a GraphStore backed by Memgraph.
// Supports Bolt (default, via neo4j-go-driver/v5 — compatible with Memgraph's
// Bolt endpoint) and HTTP transport.
// Register type: "memgraph".
//
// Metadata keys:
//
//	bolt_url  — default bolt://localhost:7687 (used when protocol=bolt)
//	http_url  — default http://localhost:7444 (used when protocol=http)
//	username  — optional
//	password  — optional
//	protocol  — "bolt" (default) or "http"
package memgraph

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"

	"github.com/google/uuid"
	neo4jdriver "github.com/neo4j/neo4j-go-driver/v5/neo4j"

	"github.com/sonicboom15/daimon/internal/memory"
)

func init() {
	memory.RegisterGraph("memgraph", func(cfg memory.StoreConfig) (memory.GraphStore, error) {
		return New(cfg)
	})
}

var validRelType = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// Store executes Cypher queries against Memgraph.
type Store struct {
	protocol string
	httpURL  string
	username string
	password string
	driver   neo4jdriver.DriverWithContext
	client   *http.Client
}

// New creates a Store from the provided config.
func New(cfg memory.StoreConfig) (*Store, error) {
	meta := cfg.Metadata

	protocol := meta["protocol"]
	if protocol == "" {
		protocol = "bolt"
	}
	boltURL := meta["bolt_url"]
	if boltURL == "" {
		boltURL = "bolt://localhost:7687"
	}
	httpURL := meta["http_url"]
	if httpURL == "" {
		httpURL = "http://localhost:7444"
	}

	s := &Store{
		protocol: protocol,
		httpURL:  httpURL,
		username: meta["username"],
		password: meta["password"],
		client:   &http.Client{},
	}

	if protocol == "bolt" {
		auth := neo4jdriver.NoAuth()
		if s.username != "" || s.password != "" {
			auth = neo4jdriver.BasicAuth(s.username, s.password, "")
		}
		driver, err := neo4jdriver.NewDriverWithContext(boltURL, auth)
		if err != nil {
			return nil, fmt.Errorf("memgraph: create driver: %w", err)
		}
		s.driver = driver
	}
	return s, nil
}

func (s *Store) execCypher(ctx context.Context, query string, params map[string]any) ([]memory.GraphNode, error) {
	if s.protocol == "bolt" {
		return s.boltCypher(ctx, query, params)
	}
	return s.httpCypher(ctx, query, params)
}

func (s *Store) boltCypher(ctx context.Context, query string, params map[string]any) ([]memory.GraphNode, error) {
	session := s.driver.NewSession(ctx, neo4jdriver.SessionConfig{})
	defer session.Close(ctx)

	result, err := session.Run(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("memgraph bolt: %w", err)
	}
	var rows []memory.GraphNode
	for result.Next(ctx) {
		row := make(memory.GraphNode)
		for _, key := range result.Record().Keys {
			row[key] = result.Record().AsMap()[key]
		}
		rows = append(rows, row)
	}
	return rows, result.Err()
}

func (s *Store) httpCypher(ctx context.Context, query string, params map[string]any) ([]memory.GraphNode, error) {
	if params == nil {
		params = map[string]any{}
	}
	body, _ := json.Marshal(map[string]any{
		"query":      query,
		"parameters": params,
	})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, s.httpURL+"/query", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if s.username != "" {
		req.SetBasicAuth(s.username, s.password)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("memgraph http: %w", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("memgraph http: status %d: %s", resp.StatusCode, b)
	}

	var result struct {
		Results []any `json:"results"`
	}
	if err := json.Unmarshal(b, &result); err != nil {
		return nil, fmt.Errorf("memgraph http: decode: %w", err)
	}
	var rows []memory.GraphNode
	for _, r := range result.Results {
		if m, ok := r.(map[string]any); ok {
			rows = append(rows, memory.GraphNode(m))
		}
	}
	return rows, nil
}

func (s *Store) AddNode(ctx context.Context, id string, labels []string, props map[string]any) (string, error) {
	if id == "" {
		id = uuid.NewString()
	}
	if props == nil {
		props = map[string]any{}
	}
	props["id"] = id

	labelStr := ""
	for _, l := range labels {
		labelStr += ":" + l
	}

	q := fmt.Sprintf("MERGE (n%s {id: $id}) SET n += $props RETURN n.id", labelStr)
	_, err := s.execCypher(ctx, q, map[string]any{"id": id, "props": props})
	if err != nil {
		return "", fmt.Errorf("memgraph: AddNode: %w", err)
	}
	return id, nil
}

func (s *Store) AddEdge(ctx context.Context, fromID, toID, relType string, props map[string]any) error {
	if !validRelType.MatchString(relType) {
		return fmt.Errorf("memgraph: invalid relationship type %q: must match [A-Za-z_][A-Za-z0-9_]*", relType)
	}
	if props == nil {
		props = map[string]any{}
	}
	q := fmt.Sprintf(`
		MATCH (a {id: $from}), (b {id: $to})
		MERGE (a)-[r:%s]->(b)
		SET r += $props`, relType)
	_, err := s.execCypher(ctx, q, map[string]any{"from": fromID, "to": toID, "props": props})
	if err != nil {
		return fmt.Errorf("memgraph: AddEdge: %w", err)
	}
	return nil
}

func (s *Store) Cypher(ctx context.Context, query string, params map[string]any) ([]memory.GraphNode, error) {
	rows, err := s.execCypher(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("memgraph: Cypher: %w", err)
	}
	return rows, nil
}

func (s *Store) Delete(ctx context.Context, id string) error {
	_, err := s.execCypher(ctx,
		"MATCH (n {id: $id}) DETACH DELETE n",
		map[string]any{"id": id})
	if err != nil {
		return fmt.Errorf("memgraph: Delete: %w", err)
	}
	return nil
}
