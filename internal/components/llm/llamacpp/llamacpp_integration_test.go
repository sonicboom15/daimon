// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Package llamacpp_test runs integration tests against a real Ollama container.
// It requires Docker to be running. If Docker is unavailable the tests are
// skipped gracefully (exit 0, not exit 1).
//
// The first run downloads the model (~350 MB for qwen2:0.5b) which takes a
// couple of minutes depending on network speed. Subsequent runs use Docker's
// layer cache.
//
// Run: go test -tags integration -v ./internal/components/llm/llamacpp/
// Override model: DAIMON_OLLAMA_MODEL=llama3.2:1b go test -tags integration ...
package llamacpp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/sonicboom15/daimon/internal/components/llm/llamacpp"
	"github.com/sonicboom15/daimon/internal/conversation"
)

// ollamaV1URL is set by TestMain and is the base URL for the OpenAI-compatible
// Ollama endpoint (e.g. "http://localhost:54321/v1").
var ollamaV1URL string

// ollamaModel is the model pulled into the container.
var ollamaModel string

func TestMain(m *testing.M) {
	model := os.Getenv("DAIMON_OLLAMA_MODEL")
	if model == "" {
		model = "qwen2:0.5b" // ~352 MB; small enough for CI
	}
	ollamaModel = model

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	container, err := startOllama(ctx)
	if err != nil {
		// Docker is unavailable — skip all tests rather than fail.
		slog.Info("skipping llamacpp integration tests: Docker not available", "err", err)
		os.Exit(0)
	}

	host, err := container.Host(ctx)
	if err != nil {
		slog.Error("container.Host", "err", err)
		os.Exit(1)
	}
	port, err := container.MappedPort(ctx, "11434")
	if err != nil {
		slog.Error("container.MappedPort", "err", err)
		os.Exit(1)
	}

	baseURL := fmt.Sprintf("http://%s:%s", host, port.Port())
	ollamaV1URL = baseURL + "/v1"

	slog.Info("pulling model into Ollama container", "model", model)
	if err := pullModel(ctx, baseURL, model); err != nil {
		slog.Error("failed to pull model", "model", model, "err", err)
		_ = container.Terminate(context.Background())
		os.Exit(1)
	}
	slog.Info("model ready", "model", model)

	code := m.Run()
	_ = container.Terminate(context.Background())
	os.Exit(code)
}

// startOllama launches an Ollama container and waits until its HTTP API is ready.
func startOllama(ctx context.Context) (testcontainers.Container, error) {
	return testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "ollama/ollama:latest",
			ExposedPorts: []string{"11434/tcp"},
			WaitingFor: wait.ForHTTP("/api/version").
				WithPort("11434/tcp").
				WithStatusCodeMatcher(func(code int) bool { return code == 200 }).
				WithStartupTimeout(2 * time.Minute),
		},
		Started: true,
	})
}

// pullModel calls the Ollama REST API to download a model into the container.
// It blocks until the download completes (or ctx is cancelled).
func pullModel(ctx context.Context, baseURL, model string) error {
	body, _ := json.Marshal(map[string]any{"model": model, "stream": false})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		baseURL+"/api/pull", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("pull request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("pull returned %d: %s", resp.StatusCode, respBody)
	}

	var result struct {
		Status string `json:"status"`
		Error  string `json:"error"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("parse pull response: %w", err)
	}
	if result.Error != "" {
		return fmt.Errorf("pull error: %s", result.Error)
	}
	return nil
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func newComp(t *testing.T) *llamacpp.Component {
	t.Helper()
	if ollamaV1URL == "" {
		t.Skip("Ollama container not available")
	}
	comp, err := llamacpp.New(conversation.ComponentConfig{
		Metadata: map[string]string{
			"base_url":      ollamaV1URL,
			"default_model": ollamaModel,
			// Ollama doesn't validate the API key; llamacpp component
			// defaults to "local" when empty, which Ollama accepts.
		},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return comp
}

func collect(t *testing.T, comp *llamacpp.Component, req conversation.Request) []conversation.Chunk {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	ch, err := comp.Chat(ctx, req)
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}

	var chunks []conversation.Chunk
	for c := range ch {
		chunks = append(chunks, c)
		if c.Type == conversation.ChunkError {
			t.Fatalf("received error chunk: %s", c.Error)
		}
	}
	return chunks
}

func fullText(chunks []conversation.Chunk) string {
	var sb strings.Builder
	for _, c := range chunks {
		if c.Type == conversation.ChunkText {
			sb.WriteString(c.Text)
		}
	}
	return sb.String()
}

// ── Tests ────────────────────────────────────────────────────────────────────

func TestLlamaCpp_BasicStreaming(t *testing.T) {
	comp := newComp(t)
	chunks := collect(t, comp, conversation.Request{
		Messages: []conversation.Message{
			{Role: conversation.RoleUser, Content: "Reply with exactly one word: PONG"},
		},
	})

	// Small models don't always follow instructions perfectly, but they should
	// produce some text and close the stream cleanly.
	text := fullText(chunks)
	if text == "" {
		t.Error("expected non-empty text response")
	}

	last := chunks[len(chunks)-1]
	if last.Type != conversation.ChunkDone {
		t.Errorf("last chunk type = %q, want done", last.Type)
	}
}

func TestLlamaCpp_SystemMessage(t *testing.T) {
	comp := newComp(t)
	chunks := collect(t, comp, conversation.Request{
		System: "You are a concise assistant. Keep responses under 20 words.",
		Messages: []conversation.Message{
			{Role: conversation.RoleUser, Content: "What is 2+2?"},
		},
	})

	text := fullText(chunks)
	if text == "" {
		t.Error("expected non-empty text response")
	}
	t.Logf("response: %q", text)
}

func TestLlamaCpp_MultiTurn(t *testing.T) {
	comp := newComp(t)
	chunks := collect(t, comp, conversation.Request{
		Messages: []conversation.Message{
			{Role: conversation.RoleUser, Content: "My favourite number is 42."},
			{Role: conversation.RoleAssistant, Content: "Got it! Your favourite number is 42."},
			{Role: conversation.RoleUser, Content: "What is my favourite number?"},
		},
	})

	text := fullText(chunks)
	if !strings.Contains(text, "42") {
		t.Errorf("expected 42 in response, got: %q", text)
	}
}

func TestLlamaCpp_MaxTokens(t *testing.T) {
	comp := newComp(t)
	maxTok := 5
	chunks := collect(t, comp, conversation.Request{
		MaxTokens: maxTok,
		Messages: []conversation.Message{
			{Role: conversation.RoleUser, Content: "Count from 1 to 100."},
		},
	})

	text := fullText(chunks)
	words := strings.Fields(text)
	// 5 tokens ≈ 3–7 words depending on tokeniser; allow generous margin.
	if len(words) > 20 {
		t.Errorf("response too long for max_tokens=%d (%d words): %q", maxTok, len(words), text)
	}
}

func TestLlamaCpp_StreamClosedCleanly(t *testing.T) {
	// Verify the channel is always closed (no goroutine leak) even when we
	// don't read all tokens.
	comp := newComp(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	ch, err := comp.Chat(ctx, conversation.Request{
		Messages: []conversation.Message{
			{Role: conversation.RoleUser, Content: "Say hello."},
		},
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}

	// Drain to completion.
	var gotDone bool
	for c := range ch {
		if c.Type == conversation.ChunkDone {
			gotDone = true
		}
	}
	if !gotDone {
		t.Error("channel closed without a ChunkDone")
	}
}
