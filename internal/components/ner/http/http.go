// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

// Package http provides an NER model driver that calls an external HTTP endpoint
// (such as a local FastAPI service, Triton Inference Server, or Hugging Face TEI).
// Registered type: "ner/http".
//
// Metadata keys:
//
//	base_url          — URL of the external service (e.g. http://localhost:8000)
//	api_key           — optional Bearer authorization token
//	timeout_ms        — HTTP client timeout in milliseconds (default 5000)
//	default_labels    — comma-separated list of default entity labels
//	default_threshold — default confidence threshold (e.g. 0.60)
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

	"github.com/sonicboom15/daimon/internal/ner"
)

func init() {
	ner.Register("ner/http", func(cfg ner.Config) (ner.Model, error) {
		return New(cfg)
	})
}

// Model implements ner.Model via HTTP calls to an external service.
type Model struct {
	endpoint         string
	apiKey           string
	defaultLabels    []string
	defaultThreshold float64
	client           *http.Client
}

// New creates an HTTP NER model driver from configuration.
func New(cfg ner.Config) (*Model, error) {
	meta := cfg.Metadata

	rawURL := strings.TrimRight(meta["base_url"], "/")
	if rawURL == "" {
		rawURL = strings.TrimRight(meta["url"], "/")
	}
	if rawURL == "" {
		return nil, fmt.Errorf("ner/http: base_url is required")
	}

	endpoint := rawURL
	if !strings.HasSuffix(endpoint, "/extract") {
		endpoint = endpoint + "/extract"
	}

	timeout := 5 * time.Second
	if tStr := meta["timeout_ms"]; tStr != "" {
		if ms, err := strconv.Atoi(tStr); err == nil && ms > 0 {
			timeout = time.Duration(ms) * time.Millisecond
		}
	}

	var defaultLabels []string
	if labelsStr := meta["default_labels"]; labelsStr != "" {
		for _, l := range strings.Split(labelsStr, ",") {
			trimmed := strings.TrimSpace(l)
			if trimmed != "" {
				defaultLabels = append(defaultLabels, trimmed)
			}
		}
	}

	var defaultThreshold float64
	if thStr := meta["default_threshold"]; thStr != "" {
		if th, err := strconv.ParseFloat(thStr, 64); err == nil {
			defaultThreshold = th
		}
	}

	return &Model{
		endpoint:         endpoint,
		apiKey:           meta["api_key"],
		defaultLabels:    defaultLabels,
		defaultThreshold: defaultThreshold,
		client:           &http.Client{Timeout: timeout},
	}, nil
}

// Extract calls the external HTTP NER endpoint.
func (m *Model) Extract(ctx context.Context, req ner.ExtractRequest) (*ner.ExtractResponse, error) {
	if len(req.Labels) == 0 && len(m.defaultLabels) > 0 {
		req.Labels = m.defaultLabels
	}
	if req.Threshold == 0 && m.defaultThreshold > 0 {
		req.Threshold = m.defaultThreshold
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("ner/http: encoding request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, m.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("ner/http: creating request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if m.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+m.apiKey)
	}

	resp, err := m.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ner/http: dispatching request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("ner/http: status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var out ner.ExtractResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("ner/http: decoding response: %w", err)
	}

	return &out, nil
}

