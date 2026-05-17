// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package openai

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
	t.Setenv("OPENAI_API_KEY", "")
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
	if c.defaultModel != "gpt-4o" {
		t.Errorf("want gpt-4o, got %s", c.defaultModel)
	}
}

func TestChat_textResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintln(w, `data: {"id":"1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}`)
		fmt.Fprintln(w)
		fmt.Fprintln(w, `data: {"id":"1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":" world"},"finish_reason":"stop"}]}`)
		fmt.Fprintln(w)
		fmt.Fprintln(w, "data: [DONE]")
	}))
	defer srv.Close()

	c, err := New(conversation.ComponentConfig{
		Metadata: map[string]string{
			"api_key":       "test-key",
			"base_url":      srv.URL + "/v1",
			"default_model": "gpt-4o-mini",
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

	combined := strings.Join(got, "")
	if combined != "Hello world" {
		t.Errorf("want 'Hello world', got %q", combined)
	}
}

func TestChat_toolCall(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		// First chunk: tool call start with id + name
		fmt.Fprintln(w, `data: {"id":"1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_abc","type":"function","function":{"name":"get_weather","arguments":""}}]},"finish_reason":null}]}`)
		fmt.Fprintln(w)
		// Second chunk: arguments fragment
		fmt.Fprintln(w, `data: {"id":"1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"city\":\"Tokyo\"}"}}]},"finish_reason":"tool_calls"}]}`)
		fmt.Fprintln(w)
		fmt.Fprintln(w, "data: [DONE]")
	}))
	defer srv.Close()

	c, _ := New(conversation.ComponentConfig{
		Metadata: map[string]string{
			"api_key":  "test-key",
			"base_url": srv.URL + "/v1",
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
	if toolChunks[0].ToolCall.ID != "call_abc" {
		t.Errorf("want call_abc, got %s", toolChunks[0].ToolCall.ID)
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
		fmt.Fprint(w, `{"error":{"message":"invalid api key","type":"invalid_request_error"}}`)
	}))
	defer srv.Close()

	c, _ := New(conversation.ComponentConfig{
		Metadata: map[string]string{
			"api_key":  "bad-key",
			"base_url": srv.URL + "/v1",
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
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintln(w, "data: [DONE]")
	}))
	defer srv.Close()

	c, _ := New(conversation.ComponentConfig{
		Metadata: map[string]string{
			"api_key":       "test-key",
			"base_url":      srv.URL + "/v1",
			"default_model": "gpt-4o-mini",
		},
	})
	ch, _ := c.Chat(context.Background(), conversation.Request{
		Messages: []conversation.Message{{Role: conversation.RoleUser, Content: "hi"}},
	})
	for range ch {
	}

	if gotModel != "gpt-4o-mini" {
		t.Errorf("want gpt-4o-mini, got %q", gotModel)
	}
}
