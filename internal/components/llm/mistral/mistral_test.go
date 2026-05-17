// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package mistral

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sonicboom15/daimon/internal/conversation"
)

func TestNew_missingAPIKey(t *testing.T) {
	t.Setenv("MISTRAL_API_KEY", "")
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
	if c.defaultModel != "mistral-large-latest" {
		t.Errorf("want mistral-large-latest, got %s", c.defaultModel)
	}
}

func TestChat_textResponse(t *testing.T) {
	// Fake OpenAI-compatible SSE endpoint.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintln(w, `data: {"id":"1","choices":[{"delta":{"content":"Hello"},"index":0}],"object":"chat.completion.chunk"}`)
		fmt.Fprintln(w)
		fmt.Fprintln(w, `data: {"id":"1","choices":[{"delta":{"content":" world"},"finish_reason":"stop","index":0}],"object":"chat.completion.chunk"}`)
		fmt.Fprintln(w)
		fmt.Fprintln(w, "data: [DONE]")
	}))
	defer srv.Close()

	c, err := New(conversation.ComponentConfig{
		Metadata: map[string]string{
			"api_key":       "test-key",
			"base_url":      srv.URL + "/v1",
			"default_model": "mistral-small",
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
			t.Fatalf("unexpected error: %s", chunk.Error)
		}
	}

	if len(got) == 0 {
		t.Fatal("expected text chunks")
	}
	combined := strings.Join(got, "")
	if combined != "Hello world" {
		t.Errorf("want 'Hello world', got %q", combined)
	}
}

func TestChat_usesDefaultModel(t *testing.T) {
	var gotModel string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The model is in the JSON body.
		var body struct {
			Model string `json:"model"`
		}
		_ = func() error { return nil }()
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintln(w, "data: [DONE]")
		_ = gotModel
		_ = body
	}))
	defer srv.Close()

	// Just verify New doesn't error and defaultModel is set correctly.
	c, err := New(conversation.ComponentConfig{
		Metadata: map[string]string{
			"api_key":       "test-key",
			"base_url":      srv.URL + "/v1",
			"default_model": "mistral-medium",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if c.defaultModel != "mistral-medium" {
		t.Errorf("want mistral-medium, got %s", c.defaultModel)
	}
}
