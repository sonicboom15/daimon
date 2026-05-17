// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package anthropic

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sonicboom15/daimon/internal/conversation"
)

func TestNew_missingAPIKey(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	_, err := New(conversation.ComponentConfig{
		Metadata: map[string]string{},
	})
	if err == nil {
		t.Fatal("expected error when api_key absent")
	}
}

func TestNew_defaults(t *testing.T) {
	c, err := New(conversation.ComponentConfig{
		Metadata: map[string]string{"api_key": "test"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if c.defaultModel != "claude-opus-4-7" {
		t.Errorf("want claude-opus-4-7, got %s", c.defaultModel)
	}
}

// anthropicSSE writes a minimal but complete Anthropic streaming response
// containing a single text block with the supplied text.
func anthropicTextSSE(w http.ResponseWriter, text string) {
	w.Header().Set("Content-Type", "text/event-stream")
	events := []string{
		fmt.Sprintf(`event: message_start`+"\n"+`data: {"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","content":[],"model":"claude-opus-4-7","stop_reason":null,"usage":{"input_tokens":1,"output_tokens":0}}}`),
		fmt.Sprintf(`event: content_block_start`+"\n"+`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`),
		fmt.Sprintf(`event: content_block_delta`+"\n"+`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":%s}}`, jsonStr(text)),
		`event: content_block_stop` + "\n" + `data: {"type":"content_block_stop","index":0}`,
		`event: message_delta` + "\n" + `data: {"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":1}}`,
		`event: message_stop` + "\n" + `data: {"type":"message_stop"}`,
	}
	for _, e := range events {
		fmt.Fprintln(w, e)
		fmt.Fprintln(w)
	}
}

func jsonStr(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func TestChat_textResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		anthropicTextSSE(w, "Hello world")
	}))
	defer srv.Close()

	c, err := New(conversation.ComponentConfig{
		Metadata: map[string]string{
			"api_key":       "test-key",
			"base_url":      srv.URL,
			"default_model": "claude-haiku-4-5-20251001",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	ch, err := c.Chat(context.Background(), conversation.Request{
		Messages: []conversation.Message{{Role: conversation.RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	var got []string
	for chunk := range ch {
		switch chunk.Type {
		case conversation.ChunkText:
			got = append(got, chunk.Text)
		case conversation.ChunkError:
			t.Fatalf("unexpected error chunk: %s", chunk.Error)
		}
	}

	if len(got) == 0 {
		t.Fatal("expected at least one text chunk")
	}
	if strings.Join(got, "") != "Hello world" {
		t.Errorf("want 'Hello world', got %q", strings.Join(got, ""))
	}
}

func TestChat_toolCall(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		events := []string{
			`event: message_start` + "\n" + `data: {"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","content":[],"model":"claude-opus-4-7","stop_reason":null,"usage":{"input_tokens":1,"output_tokens":0}}}`,
			`event: content_block_start` + "\n" + `data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_abc","name":"get_weather","input":{}}}`,
			`event: content_block_delta` + "\n" + `data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"city\":\"Tokyo\"}"}}`,
			`event: content_block_stop` + "\n" + `data: {"type":"content_block_stop","index":0}`,
			`event: message_delta` + "\n" + `data: {"type":"message_delta","delta":{"stop_reason":"tool_use","stop_sequence":null},"usage":{"output_tokens":5}}`,
			`event: message_stop` + "\n" + `data: {"type":"message_stop"}`,
		}
		for _, e := range events {
			fmt.Fprintln(w, e)
			fmt.Fprintln(w)
		}
	}))
	defer srv.Close()

	c, _ := New(conversation.ComponentConfig{
		Metadata: map[string]string{
			"api_key":  "test-key",
			"base_url": srv.URL,
		},
	})

	ch, err := c.Chat(context.Background(), conversation.Request{
		Messages: []conversation.Message{{Role: conversation.RoleUser, Content: "weather?"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	var toolChunks []conversation.Chunk
	for chunk := range ch {
		if chunk.Type == conversation.ChunkToolCall {
			toolChunks = append(toolChunks, chunk)
		}
	}

	if len(toolChunks) != 1 {
		t.Fatalf("want 1 tool call, got %d", len(toolChunks))
	}
	if toolChunks[0].ToolCall.Name != "get_weather" {
		t.Errorf("want get_weather, got %s", toolChunks[0].ToolCall.Name)
	}
	if toolChunks[0].ToolCall.ID != "toolu_abc" {
		t.Errorf("want toolu_abc, got %s", toolChunks[0].ToolCall.ID)
	}
	var input map[string]string
	if err := json.Unmarshal(toolChunks[0].ToolCall.Input, &input); err != nil {
		t.Fatalf("bad input JSON: %v", err)
	}
	if input["city"] != "Tokyo" {
		t.Errorf("want city=Tokyo, got %q", input["city"])
	}
}

func TestChat_httpError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"type":"error","error":{"type":"authentication_error","message":"invalid x-api-key"}}`)
	}))
	defer srv.Close()

	c, _ := New(conversation.ComponentConfig{
		Metadata: map[string]string{
			"api_key":  "bad-key",
			"base_url": srv.URL,
		},
	})

	ch, err := c.Chat(context.Background(), conversation.Request{
		Messages: []conversation.Message{{Role: conversation.RoleUser, Content: "hi"}},
	})
	if err != nil {
		return // error surfaced immediately — pass
	}
	for chunk := range ch {
		if chunk.Type == conversation.ChunkError {
			return // error surfaced via channel — pass
		}
	}
	t.Fatal("expected an error for 401 response")
}

func TestChat_usesDefaultModel(t *testing.T) {
	var gotModel string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Model string `json:"model"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		gotModel = body.Model
		anthropicTextSSE(w, "ok")
	}))
	defer srv.Close()

	c, _ := New(conversation.ComponentConfig{
		Metadata: map[string]string{
			"api_key":       "test-key",
			"base_url":      srv.URL,
			"default_model": "claude-haiku-4-5-20251001",
		},
	})
	ch, _ := c.Chat(context.Background(), conversation.Request{
		Messages: []conversation.Message{{Role: conversation.RoleUser, Content: "hi"}},
	})
	for range ch {
	}

	if gotModel != "claude-haiku-4-5-20251001" {
		t.Errorf("want claude-haiku-4-5-20251001, got %q", gotModel)
	}
}
