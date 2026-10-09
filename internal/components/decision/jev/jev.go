// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

// Package jev provides a Decision model driver backed by the TypeSafe/Jev API.
// Registered type: "decision/jev".
//
// Metadata keys:
//
//	api_key       — API key (falls back to JEV_API_KEY environment variable)
//	base_url      — base URL of the Jev service (default "https://api.typesafe.com/v1")
//	default_model — model identifier (default "jev-v1")
//	timeout_ms    — client timeout in milliseconds (default 1000)
package jev

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
	"time"

	"github.com/sonicboom15/daimon/internal/decision"
)

func init() {
	decision.Register("decision/jev", func(cfg decision.Config) (decision.Model, error) {
		return New(cfg)
	})
}

// Client implements decision.Model for TypeSafe/Jev.
type Client struct {
	baseURL      string
	apiKey       string
	defaultModel string
	client       *http.Client
}

// New creates a Jev decision client.
func New(cfg decision.Config) (*Client, error) {
	meta := cfg.Metadata

	apiKey := meta["api_key"]
	if apiKey == "" {
		apiKey = os.Getenv("JEV_API_KEY")
	}

	baseURL := strings.TrimRight(meta["base_url"], "/")
	if baseURL == "" {
		baseURL = "https://api.typesafe.com/v1"
	}

	model := meta["default_model"]
	if model == "" {
		model = "jev-v1"
	}

	timeout := 1000 * time.Millisecond
	if tStr := meta["timeout_ms"]; tStr != "" {
		if ms, err := strconv.Atoi(tStr); err == nil && ms > 0 {
			timeout = time.Duration(ms) * time.Millisecond
		}
	}

	return &Client{
		baseURL:      baseURL,
		apiKey:       apiKey,
		defaultModel: model,
		client:       &http.Client{Timeout: timeout},
	}, nil
}

type jevChoicePayload struct {
	Model    string   `json:"model"`
	State    string   `json:"state"`
	Question string   `json:"question"`
	Choices  []string `json:"choices"`
}

type jevVerifyPayload struct {
	Model     string `json:"model"`
	State     string `json:"state"`
	Statement string `json:"statement"`
}

// Choose evaluates choices against state using Jev.
func (c *Client) Choose(ctx context.Context, req decision.ChoiceRequest) (*decision.ChoiceResponse, error) {
	endpoint := c.baseURL + "/choose"
	payload := jevChoicePayload{
		Model:    c.defaultModel,
		State:    req.State,
		Question: req.Question,
		Choices:  req.Choices,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("decision/jev: encoding request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("decision/jev: creating request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("decision/jev: dispatching request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("decision/jev: status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var out decision.ChoiceResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decision/jev: decoding response: %w", err)
	}

	return &out, nil
}

// Verify checks assertion truth using Jev.
func (c *Client) Verify(ctx context.Context, req decision.VerifyRequest) (*decision.VerifyResponse, error) {
	endpoint := c.baseURL + "/verify"
	payload := jevVerifyPayload{
		Model:     c.defaultModel,
		State:     req.State,
		Statement: req.Statement,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("decision/jev: encoding request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("decision/jev: creating request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("decision/jev: dispatching request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("decision/jev: status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var out decision.VerifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decision/jev: decoding response: %w", err)
	}

	return &out, nil
}

