// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

// Package http provides a Decision model driver that forwards choice selection
// and assertion verification requests to an external HTTP webhook or service.
// Registered type: "decision/http".
//
// Metadata keys:
//
//	base_url   — base URL of the decision service (e.g. http://localhost:8000)
//	api_key    — optional Bearer authorization token
//	timeout_ms — HTTP client timeout in milliseconds (default 5000)
package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/sonicboom15/daimon/internal/decision"
)

func init() {
	decision.Register("decision/http", func(cfg decision.Config) (decision.Model, error) {
		return New(cfg)
	})
}

// Model implements decision.Model via HTTP endpoints.
type Model struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

// New creates an HTTP decision model driver.
func New(cfg decision.Config) (*Model, error) {
	meta := cfg.Metadata

	rawURL := strings.TrimRight(meta["base_url"], "/")
	if rawURL == "" {
		rawURL = strings.TrimRight(meta["url"], "/")
	}
	if rawURL == "" {
		return nil, fmt.Errorf("decision/http: base_url is required")
	}

	timeout := 5 * time.Second
	if tStr := meta["timeout_ms"]; tStr != "" {
		if ms, err := strconv.Atoi(tStr); err == nil && ms > 0 {
			timeout = time.Duration(ms) * time.Millisecond
		}
	}

	return &Model{
		baseURL: rawURL,
		apiKey:  meta["api_key"],
		client:  &http.Client{Timeout: timeout},
	}, nil
}

// Choose forwards ChoiceRequest to ${baseURL}/choose.
func (m *Model) Choose(ctx context.Context, req decision.ChoiceRequest) (*decision.ChoiceResponse, error) {
	endpoint := m.baseURL + "/choose"
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("decision/http: encoding request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("decision/http: creating request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if m.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+m.apiKey)
	}

	resp, err := m.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("decision/http: dispatching request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("decision/http: status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var out decision.ChoiceResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decision/http: decoding response: %w", err)
	}

	return &out, nil
}

// Verify forwards VerifyRequest to ${baseURL}/verify.
func (m *Model) Verify(ctx context.Context, req decision.VerifyRequest) (*decision.VerifyResponse, error) {
	endpoint := m.baseURL + "/verify"
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("decision/http: encoding request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("decision/http: creating request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if m.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+m.apiKey)
	}

	resp, err := m.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("decision/http: dispatching request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("decision/http: status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var out decision.VerifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decision/http: decoding response: %w", err)
	}

	return &out, nil
}

