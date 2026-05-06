// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sonicboom15/daimon/internal/conversation"
)

// fakeConversation returns a preset sequence of chunks.
type fakeConversation struct {
	// calls holds chunk sequences to return per-call. Each element is the
	// chunk list for one Chat() invocation. Subsequent calls cycle the last.
	calls [][]conversation.Chunk
	n     int
}

func (f *fakeConversation) Chat(_ context.Context, _ conversation.Request) (<-chan conversation.Chunk, error) {
	chunks := f.calls[min(f.n, len(f.calls)-1)]
	f.n++
	ch := make(chan conversation.Chunk, len(chunks))
	for _, c := range chunks {
		ch <- c
	}
	close(ch)
	return ch, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// fakeToolCaller returns a fixed result for any tool call.
type fakeToolCaller struct{ result string }

func (f *fakeToolCaller) CallTool(_ context.Context, _ string, _ json.RawMessage) (string, error) {
	return f.result, nil
}

// newTestServer builds a Server wired to a single "fake" component.
func newTestServer(conv conversation.Conversation) *Server {
	s := &Server{
		mux:        http.NewServeMux(),
		components: map[string]conversation.Conversation{"fake": conv},
		toolRoutes: make(map[string]toolCaller),
	}
	s.routes()
	return s
}

// readSSEChunks sends a POST to /v1/converse/{component} and collects all SSE chunks.
func readSSEChunks(t *testing.T, srv *Server, component string, body string) []conversation.Chunk {
	t.Helper()
	req := httptest.NewRequest("POST", "/v1/converse/"+component,
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", w.Code, w.Body.String())
	}

	var chunks []conversation.Chunk
	scanner := bufio.NewScanner(w.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var chunk conversation.Chunk
		if err := json.Unmarshal([]byte(line[6:]), &chunk); err != nil {
			t.Fatalf("unmarshal SSE chunk: %v", err)
		}
		chunks = append(chunks, chunk)
	}
	return chunks
}

func TestHandleConverse_TextOnly(t *testing.T) {
	conv := &fakeConversation{calls: [][]conversation.Chunk{{
		{Type: conversation.ChunkText, Text: "Hello"},
		{Type: conversation.ChunkText, Text: " world"},
		{Type: conversation.ChunkDone},
	}}}
	srv := newTestServer(conv)

	chunks := readSSEChunks(t, srv, "fake", `{"messages":[{"role":"user","content":"hi"}]}`)

	if len(chunks) != 3 {
		t.Fatalf("got %d chunks, want 3", len(chunks))
	}
	if chunks[0].Text != "Hello" {
		t.Errorf("chunk[0].Text = %q", chunks[0].Text)
	}
	if chunks[2].Type != conversation.ChunkDone {
		t.Errorf("last chunk type = %q, want done", chunks[2].Type)
	}
}

func TestHandleConverse_ErrorChunk(t *testing.T) {
	conv := &fakeConversation{calls: [][]conversation.Chunk{{
		{Type: conversation.ChunkError, Error: "upstream failure"},
	}}}
	srv := newTestServer(conv)

	chunks := readSSEChunks(t, srv, "fake", `{"messages":[{"role":"user","content":"hi"}]}`)

	if len(chunks) != 1 {
		t.Fatalf("got %d chunks, want 1", len(chunks))
	}
	if chunks[0].Type != conversation.ChunkError {
		t.Errorf("chunk type = %q, want error", chunks[0].Type)
	}
	if chunks[0].Error != "upstream failure" {
		t.Errorf("chunk error = %q", chunks[0].Error)
	}
}

func TestHandleConverse_UnknownComponent(t *testing.T) {
	srv := newTestServer(&fakeConversation{calls: [][]conversation.Chunk{{}}})

	req := httptest.NewRequest("POST", "/v1/converse/does-not-exist",
		strings.NewReader(`{"messages":[]}`))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestHandleConverse_BadRequestBody(t *testing.T) {
	srv := newTestServer(&fakeConversation{calls: [][]conversation.Chunk{{}}})

	req := httptest.NewRequest("POST", "/v1/converse/fake",
		strings.NewReader(`not json`))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestHandleConverse_AgenticLoop(t *testing.T) {
	toolID := "call-1"
	toolInput := json.RawMessage(`{"q":"Paris"}`)

	// First call returns a tool_call; second call returns pure text.
	conv := &fakeConversation{calls: [][]conversation.Chunk{
		{
			{Type: conversation.ChunkToolCall, ToolCall: &conversation.ToolCall{
				ID: toolID, Name: "search", Input: toolInput,
			}},
			{Type: conversation.ChunkDone},
		},
		{
			{Type: conversation.ChunkText, Text: "Paris is the capital of France."},
			{Type: conversation.ChunkDone},
		},
	}}
	srv := newTestServer(conv)
	srv.tools = []conversation.Tool{{Name: "search", Description: "web search"}}
	srv.toolRoutes["search"] = &fakeToolCaller{result: "Paris, France"}

	chunks := readSSEChunks(t, srv, "fake", `{"messages":[{"role":"user","content":"capital?"}]}`)

	// Expect: tool_call chunk, then text chunk, then done.
	types := make([]string, len(chunks))
	for i, c := range chunks {
		types[i] = string(c.Type)
	}

	if len(chunks) < 3 {
		t.Fatalf("got chunks %v, want at least [tool_call, text, done]", types)
	}
	if chunks[0].Type != conversation.ChunkToolCall {
		t.Errorf("chunks[0] = %q, want tool_call", chunks[0].Type)
	}
	if chunks[0].ToolCall.ID != toolID {
		t.Errorf("tool_call id = %q, want %q", chunks[0].ToolCall.ID, toolID)
	}
	last := chunks[len(chunks)-1]
	if last.Type != conversation.ChunkDone {
		t.Errorf("last chunk = %q, want done", last.Type)
	}
	if conv.n != 2 {
		t.Errorf("Chat called %d times, want 2 (one per agentic loop iteration)", conv.n)
	}
}

func TestHealthz(t *testing.T) {
	srv := newTestServer(&fakeConversation{})
	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if w.Body.String() != "ok" {
		t.Errorf("body = %q, want %q", w.Body.String(), "ok")
	}
}
