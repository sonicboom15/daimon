// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package openai_test

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sonicboom15/daimon/internal/components/llm/openai"
	"github.com/sonicboom15/daimon/internal/conversation"
)

const integrationModel = "gpt-4o-mini"

func newComp(t *testing.T) *openai.Component {
	t.Helper()
	key := os.Getenv("OPENAI_API_KEY")
	if key == "" {
		t.Skip("OPENAI_API_KEY not set — skipping OpenAI integration tests")
	}
	comp, err := openai.New(conversation.ComponentConfig{
		Metadata: map[string]string{
			"api_key":       key,
			"default_model": integrationModel,
		},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return comp
}

func collect(t *testing.T, comp *openai.Component, req conversation.Request) []conversation.Chunk {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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

// ── Basic streaming ──────────────────────────────────────────────────────────

func TestOpenAI_BasicStreaming(t *testing.T) {
	comp := newComp(t)
	chunks := collect(t, comp, conversation.Request{
		Messages: []conversation.Message{
			{Role: conversation.RoleUser, Content: "Reply with exactly one word: PONG"},
		},
	})

	text := fullText(chunks)
	if !strings.Contains(strings.ToUpper(text), "PONG") {
		t.Errorf("expected PONG in response, got: %q", text)
	}

	last := chunks[len(chunks)-1]
	if last.Type != conversation.ChunkDone {
		t.Errorf("last chunk type = %q, want done", last.Type)
	}
}

// ── System message ───────────────────────────────────────────────────────────

func TestOpenAI_SystemMessage(t *testing.T) {
	comp := newComp(t)
	chunks := collect(t, comp, conversation.Request{
		System: "You are a robot. Every response must end with the word BEEP.",
		Messages: []conversation.Message{
			{Role: conversation.RoleUser, Content: "Say hello."},
		},
	})

	text := fullText(chunks)
	if !strings.Contains(strings.ToUpper(text), "BEEP") {
		t.Errorf("expected BEEP in response (system instruction), got: %q", text)
	}
}

// ── Defaults: max_tokens ─────────────────────────────────────────────────────

func TestOpenAI_MaxTokensDefault(t *testing.T) {
	key := os.Getenv("OPENAI_API_KEY")
	if key == "" {
		t.Skip("OPENAI_API_KEY not set")
	}
	maxTok := 5
	comp, err := openai.New(conversation.ComponentConfig{
		Metadata: map[string]string{
			"api_key":       key,
			"default_model": integrationModel,
		},
		Defaults: conversation.ComponentDefaults{MaxTokens: maxTok},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	chunks := collect(t, comp, conversation.Request{
		Messages: []conversation.Message{
			{Role: conversation.RoleUser, Content: "Count from 1 to 100."},
		},
	})

	text := fullText(chunks)
	words := strings.Fields(text)
	if len(words) > 15 { // generous bound — 5 tokens is ~3–5 words
		t.Errorf("response too long for max_tokens=%d: %q", maxTok, text)
	}
}

// ── Tool calls ───────────────────────────────────────────────────────────────

var weatherTool = conversation.Tool{
	Name:        "get_current_weather",
	Description: "Get the current weather for a city.",
	InputSchema: json.RawMessage(`{
		"type": "object",
		"properties": {
			"city": {"type": "string", "description": "The city name"}
		},
		"required": ["city"]
	}`),
}

func TestOpenAI_ToolCall(t *testing.T) {
	comp := newComp(t)
	chunks := collect(t, comp, conversation.Request{
		Messages: []conversation.Message{
			{Role: conversation.RoleUser, Content: "What is the current weather in Tokyo?"},
		},
		Tools: []conversation.Tool{weatherTool},
	})

	var toolCalls []conversation.ToolCall
	for _, c := range chunks {
		if c.Type == conversation.ChunkToolCall {
			toolCalls = append(toolCalls, *c.ToolCall)
		}
	}

	if len(toolCalls) == 0 {
		t.Fatalf("expected at least one tool_call chunk, got none; full text: %q", fullText(chunks))
	}

	tc := toolCalls[0]
	if tc.Name != "get_current_weather" {
		t.Errorf("tool name = %q, want get_current_weather", tc.Name)
	}
	if !json.Valid(tc.Input) {
		t.Errorf("tool input is not valid JSON: %s", tc.Input)
	}

	var args map[string]any
	_ = json.Unmarshal(tc.Input, &args)
	city, _ := args["city"].(string)
	if !strings.Contains(strings.ToLower(city), "tokyo") {
		t.Errorf("tool input city = %q, want to contain 'tokyo'", city)
	}
}

// ── Multi-turn conversation ──────────────────────────────────────────────────

func TestOpenAI_MultiTurn(t *testing.T) {
	comp := newComp(t)
	chunks := collect(t, comp, conversation.Request{
		Messages: []conversation.Message{
			{Role: conversation.RoleUser, Content: "My name is Zephyr. Remember it."},
			{Role: conversation.RoleAssistant, Content: "Got it! I'll remember that your name is Zephyr."},
			{Role: conversation.RoleUser, Content: "What is my name?"},
		},
	})

	text := fullText(chunks)
	if !strings.Contains(strings.ToLower(text), "zephyr") {
		t.Errorf("expected name in response, got: %q", text)
	}
}
