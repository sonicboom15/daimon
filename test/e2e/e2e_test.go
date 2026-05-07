// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Package e2e runs end-to-end tests against a real daimon sidecar backed by an
// Ollama container. It requires Docker to be running; if Docker is unavailable
// the tests are skipped gracefully (exit 0, not exit 1).
//
// The first run downloads the model (~350 MB for qwen2.5:1.5b). Subsequent runs
// use Docker's layer cache.
//
// Run all e2e tests (Go + Python SDK + TypeScript SDK):
//
//	go test -tags integration -v -timeout 20m ./test/e2e/
//
// Override the model:
//
//	DAIMON_OLLAMA_MODEL=llama3.2:1b go test -tags integration -timeout 20m ./test/e2e/
package e2e_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	// Blank imports register component factories in their respective registries.
	_ "github.com/sonicboom15/daimon/internal/components/llm/llamacpp"
	_ "github.com/sonicboom15/daimon/internal/components/vector/inmemory"

	"github.com/sonicboom15/daimon/internal/conversation"
	"github.com/sonicboom15/daimon/internal/memory"
	"github.com/sonicboom15/daimon/internal/server"
	"github.com/sonicboom15/daimon/internal/session"
)

const (
	component = "llama"
	memStore  = "mem" // inmemory vector store name used in all memory tests
)

var (
	baseURL     string
	ollamaModel = "qwen2.5:1.5b"
)

func TestMain(m *testing.M) {
	if model := os.Getenv("DAIMON_OLLAMA_MODEL"); model != "" {
		ollamaModel = model
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	container, err := startOllama(ctx)
	if err != nil {
		slog.Info("skipping e2e tests: Docker not available", "err", err)
		os.Exit(0)
	}

	host, err := container.Host(ctx)
	if err != nil {
		slog.Error("container.Host", "err", err)
		container.Terminate(context.Background()) //nolint:errcheck
		os.Exit(1)
	}
	port, err := container.MappedPort(ctx, "11434")
	if err != nil {
		slog.Error("container.MappedPort", "err", err)
		container.Terminate(context.Background()) //nolint:errcheck
		os.Exit(1)
	}

	containerBase := fmt.Sprintf("http://%s:%s", host, port.Port())
	ollamaV1URL := containerBase + "/v1"

	slog.Info("pulling model", "model", ollamaModel)
	if err := pullModel(ctx, containerBase, ollamaModel); err != nil {
		slog.Error("failed to pull model", "model", ollamaModel, "err", err)
		container.Terminate(context.Background()) //nolint:errcheck
		os.Exit(1)
	}
	slog.Info("model ready", "model", ollamaModel)

	comp, err := conversation.New("llamacpp", conversation.ComponentConfig{
		Metadata: map[string]string{
			"base_url":      ollamaV1URL,
			"default_model": ollamaModel,
		},
	})
	if err != nil {
		slog.Error("creating llamacpp component", "err", err)
		container.Terminate(context.Background()) //nolint:errcheck
		os.Exit(1)
	}

	// Inmemory vector store — no external service needed.
	ms, err := memory.New("inmemory", memory.StoreConfig{Metadata: map[string]string{}})
	if err != nil {
		slog.Error("creating inmemory store", "err", err)
		container.Terminate(context.Background()) //nolint:errcheck
		os.Exit(1)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		slog.Error("net.Listen", "err", err)
		container.Terminate(context.Background()) //nolint:errcheck
		os.Exit(1)
	}

	daimonSrv := &http.Server{
		Handler: server.New(
			map[string]conversation.Conversation{component: comp},
			nil, // no MCP servers
			map[string]memory.MemoryStore{memStore: ms},
			map[string]memory.GraphStore{},
			map[string]string{}, // no RAG wiring
			session.NewInMemory(),
		),
	}
	go daimonSrv.Serve(ln) //nolint:errcheck

	baseURL = "http://" + ln.Addr().String()
	slog.Info("daimon e2e server started", "url", baseURL)

	code := m.Run()

	daimonSrv.Shutdown(context.Background()) //nolint:errcheck
	container.Terminate(context.Background()) //nolint:errcheck
	os.Exit(code)
}

// ── Healthz ───────────────────────────────────────────────────────────────────

func TestE2E_Healthz(t *testing.T) {
	resp, err := http.Get(baseURL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if strings.TrimSpace(string(body)) != "ok" {
		t.Errorf("body = %q, want \"ok\"", body)
	}
}

// ── LLM / converse ────────────────────────────────────────────────────────────

func TestE2E_BasicConverse(t *testing.T) {
	text := converseText(t, map[string]any{
		"messages": []map[string]any{
			{"role": "user", "content": "Reply with exactly one word: PONG"},
		},
	})
	if text == "" {
		t.Error("expected non-empty text response")
	}
	t.Logf("response: %q", text)
}

func TestE2E_SessionRecall(t *testing.T) {
	sessionID := "e2e-go-session-recall"
	defer deleteSession(t, sessionID)

	converseText(t, map[string]any{
		"session_id": sessionID,
		"messages": []map[string]any{
			{"role": "user", "content": "My favourite colour is blue."},
		},
	})

	reply := converseText(t, map[string]any{
		"session_id": sessionID,
		"messages": []map[string]any{
			{"role": "user", "content": "What colour did I just tell you is my favourite?"},
		},
	})

	if !strings.Contains(strings.ToLower(reply), "blue") {
		t.Errorf("expected 'blue' in session recall response, got: %q", reply)
	}
}

func TestE2E_DeleteSession(t *testing.T) {
	sessionID := "e2e-go-session-delete"

	converseText(t, map[string]any{
		"session_id": sessionID,
		"messages": []map[string]any{
			{"role": "user", "content": "Hello."},
		},
	})

	deleteSession(t, sessionID)
}

// ── Memory store (inmemory, BM25) ─────────────────────────────────────────────

func TestE2E_MemoryUpsertWithID(t *testing.T) {
	body, _ := json.Marshal(map[string]any{
		"content":  "The Eiffel Tower is 330 metres tall.",
		"metadata": map[string]string{"src": "wiki"},
	})
	resp, err := doRequest(t, http.MethodPut, baseURL+"/v1/memory/"+memStore+"/tower", body)
	if err != nil {
		t.Fatalf("PUT memory: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, body = %s", resp.StatusCode, b)
	}
	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if result["id"] != "tower" {
		t.Errorf("id = %q, want tower", result["id"])
	}
}

func TestE2E_MemoryUpsertServerAssignsID(t *testing.T) {
	body, _ := json.Marshal(map[string]any{
		"content": "The Seine is the main river of Paris.",
	})
	resp, err := doRequest(t, http.MethodPost, baseURL+"/v1/memory/"+memStore, body)
	if err != nil {
		t.Fatalf("POST memory: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, body = %s", resp.StatusCode, b)
	}
	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if result["id"] == "" {
		t.Error("expected non-empty assigned id")
	}
	t.Logf("server-assigned id = %q", result["id"])
}

func TestE2E_MemoryQuery(t *testing.T) {
	// Seed a known document.
	seed, _ := json.Marshal(map[string]any{"content": "Paris is the capital of France."})
	resp, err := doRequest(t, http.MethodPut, baseURL+"/v1/memory/"+memStore+"/paris-capital", seed)
	if err != nil {
		t.Fatalf("upsert seed: %v", err)
	}
	resp.Body.Close()

	// Query for it.
	qbody, _ := json.Marshal(map[string]any{"query": "capital of France", "top_k": 5})
	qresp, err := doRequest(t, http.MethodPost, baseURL+"/v1/memory/"+memStore+"/query", qbody)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer qresp.Body.Close()

	if qresp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(qresp.Body)
		t.Fatalf("query status = %d, body = %s", qresp.StatusCode, b)
	}

	var result struct {
		Results []struct {
			ID      string  `json:"id"`
			Content string  `json:"content"`
			Score   float64 `json:"score"`
		} `json:"results"`
	}
	if err := json.NewDecoder(qresp.Body).Decode(&result); err != nil {
		t.Fatalf("decode query response: %v", err)
	}
	if len(result.Results) == 0 {
		t.Fatal("expected at least one result")
	}

	found := false
	for _, r := range result.Results {
		if r.ID == "paris-capital" {
			found = true
			t.Logf("found doc score=%.4f content=%q", r.Score, r.Content)
			break
		}
	}
	if !found {
		t.Errorf("paris-capital not in results: %+v", result.Results)
	}
}

func TestE2E_MemoryDelete(t *testing.T) {
	// Upsert then delete.
	body, _ := json.Marshal(map[string]any{"content": "temporary document"})
	upsert, _ := doRequest(t, http.MethodPut, baseURL+"/v1/memory/"+memStore+"/to-delete", body)
	upsert.Body.Close()

	del, err := doRequest(t, http.MethodDelete, baseURL+"/v1/memory/"+memStore+"/to-delete", nil)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	del.Body.Close()

	if del.StatusCode != http.StatusNoContent {
		t.Errorf("DELETE status = %d, want 204", del.StatusCode)
	}
}

func TestE2E_MemoryUnknownStore(t *testing.T) {
	body, _ := json.Marshal(map[string]any{"query": "anything", "top_k": 1})
	resp, err := doRequest(t, http.MethodPost, baseURL+"/v1/memory/no-such-store/query", body)
	if err != nil {
		t.Fatalf("query unknown store: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

// ── SDK subprocess tests ──────────────────────────────────────────────────────

func TestE2E_PythonSDK(t *testing.T) {
	python, err := exec.LookPath("python")
	if err != nil {
		t.Skip("python not in PATH; skipping Python SDK e2e tests")
	}

	sdkDir := repoPath(t, "sdk/python")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, python, "-m", "pytest", "tests/test_e2e.py", "-v", "--tb=short")
	cmd.Dir = sdkDir
	cmd.Env = append(os.Environ(),
		"DAIMON_E2E=1",
		"DAIMON_BASE_URL="+baseURL,
		"DAIMON_MEM_STORE="+memStore,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("Python SDK e2e tests failed: %v", err)
	}
}

func TestE2E_TypeScriptSDK(t *testing.T) {
	npm, err := exec.LookPath("npm")
	if err != nil {
		t.Skip("npm not in PATH; skipping TypeScript SDK e2e tests")
	}

	sdkDir := repoPath(t, "sdk/typescript")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, npm, "run", "test:e2e")
	cmd.Dir = sdkDir
	cmd.Env = append(os.Environ(),
		"DAIMON_E2E=1",
		"DAIMON_BASE_URL="+baseURL,
		"DAIMON_MEM_STORE="+memStore,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("TypeScript SDK e2e tests failed: %v", err)
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func converseText(t *testing.T, body map[string]any) string {
	t.Helper()

	data, _ := json.Marshal(body)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		baseURL+"/v1/converse/"+component, bytes.NewReader(data))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("converse: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("converse returned %d: %s", resp.StatusCode, b)
	}

	var sb strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var chunk struct {
			Type  string `json:"type"`
			Text  string `json:"text"`
			Error string `json:"error"`
		}
		if err := json.Unmarshal([]byte(line[6:]), &chunk); err != nil {
			continue
		}
		switch chunk.Type {
		case "text":
			sb.WriteString(chunk.Text)
		case "error":
			t.Fatalf("error chunk: %s", chunk.Error)
		case "done":
			return sb.String()
		}
	}
	return sb.String()
}

func deleteSession(t *testing.T, sessionID string) {
	t.Helper()

	req, err := http.NewRequest(http.MethodDelete, baseURL+"/v1/sessions/"+sessionID, nil)
	if err != nil {
		t.Fatalf("build delete request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE session: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("DELETE session status = %d, want 204", resp.StatusCode)
	}
}

func doRequest(t *testing.T, method, url string, body []byte) (*http.Response, error) {
	t.Helper()
	var r io.Reader
	if body != nil {
		r = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, url, r)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return http.DefaultClient.Do(req)
}

func repoPath(t *testing.T, rel string) string {
	t.Helper()
	_, testFile, _, ok := runtime.Caller(1)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(testFile), "../..", rel)
}

// ── Ollama container helpers ──────────────────────────────────────────────────
// Self-contained so this package has no dependency on llamacpp's test helpers.

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
