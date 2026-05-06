// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

// Package conversation defines the core inference interface used by all
// provider components.
package conversation

import (
	"context"
	"encoding/json"
)

// Role identifies the speaker of a message.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool" // carries the result of a tool call back to the model
)

// Tool is a callable function the model may invoke.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema"` // JSON Schema object
}

// ToolCall is a request from the model to invoke a named tool.
type ToolCall struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"` // JSON-encoded arguments
}

// Message is a single turn in a conversation.
type Message struct {
	Role       Role       `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`   // set when Role == RoleAssistant and model requested tools
	ToolCallID string     `json:"tool_call_id,omitempty"` // set when Role == RoleTool
}

// Request is a provider-agnostic chat completion request.
// All fields are optional; zero/nil means "use the component's configured default."
// Provider-specific fields (TopK, FrequencyPenalty, PresencePenalty, Seed) are
// silently ignored by providers that don't support them.
type Request struct {
	Model   string    `json:"model"`
	Messages []Message `json:"messages"`
	Tools   []Tool    `json:"tools,omitempty"`
	System  string   `json:"system,omitempty"` // convenience: prepended as a system message

	// Sampling parameters
	MaxTokens        int      `json:"max_tokens,omitempty"`
	Temperature      *float64 `json:"temperature,omitempty"`
	TopP             *float64 `json:"top_p,omitempty"`
	TopK             *int64   `json:"top_k,omitempty"`              // Anthropic / llama.cpp
	Stop             []string `json:"stop,omitempty"`               // stop sequences
	FrequencyPenalty *float64 `json:"frequency_penalty,omitempty"`  // OpenAI / llama.cpp
	PresencePenalty  *float64 `json:"presence_penalty,omitempty"`   // OpenAI / llama.cpp
	Seed             *int64   `json:"seed,omitempty"`               // OpenAI / llama.cpp
}

// ChunkType classifies a streaming response chunk.
type ChunkType string

const (
	// ChunkText carries a text fragment from the model.
	ChunkText ChunkType = "text"
	// ChunkToolCall signals that the model wants to invoke a tool.
	// The ToolCall field holds the complete (accumulated) call.
	// Providers emit one ChunkToolCall per tool call, after the stream ends.
	ChunkToolCall ChunkType = "tool_call"
	// ChunkError signals a terminal error; no further chunks follow.
	ChunkError ChunkType = "error"
	// ChunkDone signals the end of a successful stream.
	ChunkDone ChunkType = "done"
)

// Chunk is a single unit of a streaming response.
type Chunk struct {
	Type     ChunkType `json:"type"`
	Text     string    `json:"text,omitempty"`
	ToolCall *ToolCall `json:"tool_call,omitempty"`
	Error    string    `json:"error,omitempty"`
}

// Conversation is the interface all provider components must implement.
// Chat begins a streaming inference call and returns a channel of Chunks.
// The channel is closed after a ChunkDone or ChunkError chunk is emitted.
// When tools are provided and the model invokes them, providers emit
// ChunkToolCall chunks before ChunkDone; the server handler drives the
// agentic loop.
// Implementations must stop and clean up when ctx is canceled.
type Conversation interface {
	Chat(ctx context.Context, req Request) (<-chan Chunk, error)
}
