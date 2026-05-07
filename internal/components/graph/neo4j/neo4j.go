// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

// Package neo4j provides a GraphStore backed by Neo4j.
// Supports Bolt (default, via neo4j-go-driver/v5) and HTTP transport.
// Register type: "neo4j".
//
// Metadata keys:
//
//	bolt_url  — default bolt://localhost:7687 (used when protocol=bolt)
//	http_url  — default http://localhost:7474 (used when protocol=http)
//	database  — Neo4j database name, default neo4j
//	username  — default neo4j
//	password  — required
//	protocol  — "bolt" (default) or "http"
package neo4j

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
	memory.RegisterGraph("neo4j", func(cfg memory.StoreConfig) (memory.GraphStore, error) {
		return New(cfg)
	})
}

var validRelType = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// Store executes Cypher queries against Neo4j.
type Store struct {
	protocol string
	httpURL  string
	database string
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
		httpURL = "http://localhost:7474"
	}
	database := meta["database"]
	if database == "" {
		database = "neo4j"
	}
	username := meta["username"]
	if username == "" {
		username = "neo4j"
	}
	password := meta["password"]

	s := &Store{
		protocol: protocol,
		httpURL:  httpURL,
		database: database,
		username: username,
		password: password,
		client:   &http.Client{},
	}

	if protocol == "bolt" {
		driver, err := neo4jdriver.NewDriverWithContext(boltURL,
			neo4jdriver.BasicAuth(username, password, ""))
		if err != nil {
			return nil, fmt.Errorf("neo4j: create driver: %w", err)
		}
		s.driver = driver
	}
	return s, nil
}

// -- Cypher helpers ----------------------------------------------------------

func (s *Store) execCypher(ctx context.Context, query string, params map[string]any) ([]memory.GraphNode, error) {
	if s.protocol == "bolt" {
		return s.boltCypher(ctx, query, params)
	}
	return s.httpCypher(ctx, query, params)
}

func (s *Store) boltCypher(ctx context.Context, query string, params map[string]any) ([]memory.GraphNode, error) {
	session := s.driver.NewSession(ctx, neo4jdriver.SessionConfig{DatabaseName: s.database})
	defer session.Close(ctx)

	result, err := session.Run(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("neo4j bolt: %w", err)
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
		"statements": []map[string]any{
			{"statement": query, "parameters": params},
		},
	})
	url := fmt.Sprintf("%s/db/%s/tx/commit", s.httpURL, s.database)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if s.username != "" {
		req.SetBasicAuth(s.username, s.password)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("neo4j http: %w", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("neo4j http: status %d: %s", resp.StatusCode, b)
	}

	var result struct {
		Results []struct {
			Columns []string         `json:"columns"`
			Data    []map[string]any `json:"data"`
		} `json:"results"`
		Errors []map[string]any `json:"errors"`
	}
	if err := json.Unmarshal(b, &result); err != nil {
		return nil, fmt.Errorf("neo4j http: decode: %w", err)
	}
	if len(result.Errors) > 0 {
		return nil, fmt.Errorf("neo4j http: cypher error: %v", result.Errors)
	}
	if len(result.Results) == 0 {
		return nil, nil
	}

	cols := result.Results[0].Columns
	var rows []memory.GraphNode
	for _, dataRow := range result.Results[0].Data {
		row := make(memory.GraphNode)
		rowVals, _ := dataRow["row"].([]any)
		for i, col := range cols {
			if i < len(rowVals) {
				row[col] = rowVals[i]
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// -- GraphStore implementation -----------------------------------------------

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
		return "", fmt.Errorf("neo4j: AddNode: %w", err)
	}
	return id, nil
}

func (s *Store) AddEdge(ctx context.Context, fromID, toID, relType string, props map[string]any) error {
	if !validRelType.MatchString(relType) {
		return fmt.Errorf("neo4j: invalid relationship type %q: must match [A-Za-z_][A-Za-z0-9_]*", relType)
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
		return fmt.Errorf("neo4j: AddEdge: %w", err)
	}
	return nil
}

func (s *Store) Cypher(ctx context.Context, query string, params map[string]any) ([]memory.GraphNode, error) {
	rows, err := s.execCypher(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("neo4j: Cypher: %w", err)
	}
	return rows, nil
}

func (s *Store) Delete(ctx context.Context, id string) error {
	_, err := s.execCypher(ctx,
		"MATCH (n {id: $id}) DETACH DELETE n",
		map[string]any{"id": id})
	if err != nil {
		return fmt.Errorf("neo4j: Delete: %w", err)
	}
	return nil
}
