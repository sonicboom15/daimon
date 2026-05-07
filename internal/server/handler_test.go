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
	"github.com/sonicboom15/daimon/internal/memory"
	"github.com/sonicboom15/daimon/internal/session"
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

// testServer bundles a Server with its underlying in-memory session store for
// direct inspection in tests.
type testServer struct {
	*Server
	sess *session.InMemory
}

// newTestServer builds a Server wired to a single "fake" component.
func newTestServer(conv conversation.Conversation) *testServer {
	sess := session.NewInMemory()
	s := &Server{
		mux:             http.NewServeMux(),
		components:      map[string]conversation.Conversation{"fake": conv},
		stores:          make(map[string]memory.MemoryStore),
		graphs:          make(map[string]memory.GraphStore),
		componentStores: make(map[string]string),
		toolRoutes:      make(map[string]toolCaller),
		storeRoutes:     make(map[string]memory.MemoryStore),
		graphRoutes:     make(map[string]memory.GraphStore),
		sessions:        sess,
	}
	s.routes()
	return &testServer{Server: s, sess: sess}
}

// recordingConversation is like fakeConversation but also captures each Chat request.
type recordingConversation struct {
	calls    [][]conversation.Chunk
	n        int
	recorded []conversation.Request
}

func (r *recordingConversation) Chat(_ context.Context, req conversation.Request) (<-chan conversation.Chunk, error) {
	r.recorded = append(r.recorded, req)
	chunks := r.calls[min(r.n, len(r.calls)-1)]
	r.n++
	ch := make(chan conversation.Chunk, len(chunks))
	for _, c := range chunks {
		ch <- c
	}
	close(ch)
	return ch, nil
}

// readSSEChunks sends a POST to /v1/converse/{component} and collects all SSE chunks.
func readSSEChunks(t *testing.T, srv *testServer, component string, body string) []conversation.Chunk {
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

func TestSession_HistoryStoredAfterFirstRequest(t *testing.T) {
	conv := &recordingConversation{calls: [][]conversation.Chunk{{
		{Type: conversation.ChunkText, Text: "Hi there!"},
		{Type: conversation.ChunkDone},
	}}}
	srv := newTestServer(conv)

	readSSEChunks(t, srv, "fake", `{"session_id":"s1","messages":[{"role":"user","content":"Hello"}]}`)

	history, err := srv.sess.Get(context.Background(), "s1")
	if err != nil {
		t.Fatalf("Get session: %v", err)
	}
	if len(history) == 0 {
		t.Fatal("session not stored after first request")
	}
	if len(history) != 2 {
		t.Fatalf("history len = %d, want 2 (user + assistant)", len(history))
	}
	if history[0].Role != conversation.RoleUser || history[0].Content != "Hello" {
		t.Errorf("history[0] = %+v, want user:Hello", history[0])
	}
	if history[1].Role != conversation.RoleAssistant || history[1].Content != "Hi there!" {
		t.Errorf("history[1] = %+v, want assistant:Hi there!", history[1])
	}
}

func TestSession_HistoryPrependedOnSecondRequest(t *testing.T) {
	conv := &recordingConversation{calls: [][]conversation.Chunk{
		{{Type: conversation.ChunkText, Text: "Hi!"}, {Type: conversation.ChunkDone}},
		{{Type: conversation.ChunkText, Text: "Sure!"}, {Type: conversation.ChunkDone}},
	}}
	srv := newTestServer(conv)

	readSSEChunks(t, srv, "fake", `{"session_id":"s1","messages":[{"role":"user","content":"Hello"}]}`)
	readSSEChunks(t, srv, "fake", `{"session_id":"s1","messages":[{"role":"user","content":"Follow-up"}]}`)

	if len(conv.recorded) < 2 {
		t.Fatalf("Chat called %d times, want at least 2", len(conv.recorded))
	}
	msgs := conv.recorded[1].Messages
	if len(msgs) != 3 {
		t.Fatalf("second Chat got %d messages, want 3", len(msgs))
	}
	if msgs[0].Content != "Hello" {
		t.Errorf("msgs[0].Content = %q, want Hello", msgs[0].Content)
	}
	if msgs[1].Role != conversation.RoleAssistant || msgs[1].Content != "Hi!" {
		t.Errorf("msgs[1] = %+v, want assistant:Hi!", msgs[1])
	}
	if msgs[2].Content != "Follow-up" {
		t.Errorf("msgs[2].Content = %q, want Follow-up", msgs[2].Content)
	}
}

func TestSession_SessionIDStrippedFromChatRequest(t *testing.T) {
	conv := &recordingConversation{calls: [][]conversation.Chunk{{
		{Type: conversation.ChunkDone},
	}}}
	srv := newTestServer(conv)

	readSSEChunks(t, srv, "fake", `{"session_id":"s1","messages":[]}`)

	if len(conv.recorded) == 0 {
		t.Fatal("Chat was not called")
	}
	if conv.recorded[0].SessionID != "" {
		t.Errorf("SessionID %q was forwarded to Chat; want it stripped", conv.recorded[0].SessionID)
	}
}

func TestSession_NoSessionIDIsStateless(t *testing.T) {
	conv := &recordingConversation{calls: [][]conversation.Chunk{{
		{Type: conversation.ChunkText, Text: "hello"},
		{Type: conversation.ChunkDone},
	}}}
	srv := newTestServer(conv)

	readSSEChunks(t, srv, "fake", `{"messages":[{"role":"user","content":"hi"}]}`)

	if srv.sess.Len() != 0 {
		t.Errorf("session count = %d, want 0 for stateless request", srv.sess.Len())
	}
}

func TestSession_DeleteClearsHistory(t *testing.T) {
	srv := newTestServer(&fakeConversation{calls: [][]conversation.Chunk{{}}})
	_ = srv.sess.Set(context.Background(), "s1", []conversation.Message{{Role: conversation.RoleUser, Content: "hi"}})

	req := httptest.NewRequest("DELETE", "/v1/sessions/s1", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("DELETE status = %d, want 204", w.Code)
	}
	msgs, _ := srv.sess.Get(context.Background(), "s1")
	if msgs != nil {
		t.Error("session still present after DELETE")
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
