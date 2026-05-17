// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sonicboom15/daimon/internal/conversation"
)

func TestNew_missingAPIKey(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("GOOGLE_API_KEY", "")
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
	if c.defaultModel != "gemini-2.0-flash" {
		t.Errorf("want gemini-2.0-flash, got %s", c.defaultModel)
	}
}

func TestChat_textResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-goog-api-key") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintln(w, `data: {"candidates":[{"content":{"parts":[{"text":"Hello"}],"role":"model"}}]}`)
		fmt.Fprintln(w)
		fmt.Fprintln(w, `data: {"candidates":[{"content":{"parts":[{"text":" world"}],"role":"model"},"finishReason":"STOP"}]}`)
		fmt.Fprintln(w)
	}))
	defer srv.Close()

	c, _ := New(conversation.ComponentConfig{
		Metadata: map[string]string{
			"api_key":  "test-key",
			"base_url": srv.URL,
		},
	})

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

	if len(got) != 2 || got[0] != "Hello" || got[1] != " world" {
		t.Errorf("unexpected chunks: %v", got)
	}
}

func TestChat_toolCall(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		event := map[string]any{
			"candidates": []map[string]any{{
				"content": map[string]any{
					"role": "model",
					"parts": []map[string]any{{
						"functionCall": map[string]any{
							"name": "get_weather",
							"args": map[string]any{"city": "Tokyo"},
						},
					}},
				},
				"finishReason": "STOP",
			}},
		}
		b, _ := json.Marshal(event)
		fmt.Fprintf(w, "data: %s\n\n", b)
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
	if toolChunks[0].ToolCall.ID != "call_0" {
		t.Errorf("want call_0, got %s", toolChunks[0].ToolCall.ID)
	}
}

func TestChat_errorEvent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintln(w, `data: {"error":{"code":400,"message":"bad request"}}`)
		fmt.Fprintln(w)
	}))
	defer srv.Close()

	c, _ := New(conversation.ComponentConfig{
		Metadata: map[string]string{
			"api_key":  "test-key",
			"base_url": srv.URL,
		},
	})

	ch, err := c.Chat(context.Background(), conversation.Request{
		Messages: []conversation.Message{{Role: conversation.RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	for chunk := range ch {
		if chunk.Type == conversation.ChunkError {
			return // expected
		}
	}
	t.Fatal("expected error chunk")
}

func TestChat_httpError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":{"code":401,"message":"invalid key"}}`)
	}))
	defer srv.Close()

	c, _ := New(conversation.ComponentConfig{
		Metadata: map[string]string{
			"api_key":  "bad-key",
			"base_url": srv.URL,
		},
	})

	_, err := c.Chat(context.Background(), conversation.Request{
		Messages: []conversation.Message{{Role: conversation.RoleUser, Content: "hi"}},
	})
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
}

func TestBuildContents_toolRoundTrip(t *testing.T) {
	msgs := []conversation.Message{
		{Role: conversation.RoleUser, Content: "weather?"},
		{
			Role: conversation.RoleAssistant,
			ToolCalls: []conversation.ToolCall{
				{ID: "call_0", Name: "get_weather", Input: json.RawMessage(`{"city":"Tokyo"}`)},
			},
		},
		{Role: conversation.RoleTool, Content: "Sunny 25°C", ToolCallID: "call_0"},
	}

	contents, err := buildContents(msgs)
	if err != nil {
		t.Fatal(err)
	}
	// Expected: user, model (with functionCall), user (with functionResponse)
	if len(contents) != 3 {
		t.Fatalf("want 3 contents, got %d", len(contents))
	}
	if contents[2].Role != "user" {
		t.Errorf("want user role for tool response, got %s", contents[2].Role)
	}
	if contents[2].Parts[0].FunctionResponse == nil {
		t.Fatal("expected functionResponse part")
	}
	if contents[2].Parts[0].FunctionResponse.Name != "get_weather" {
		t.Errorf("want get_weather, got %s", contents[2].Parts[0].FunctionResponse.Name)
	}
}
