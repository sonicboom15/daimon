// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

// Package conversation defines the core inference interface used by all
// provider components.
package conversation

import "context"

// Role identifies the speaker of a message.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// Message is a single turn in a conversation.
type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

// Request is a provider-agnostic chat completion request.
type Request struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Temperature *float64  `json:"temperature,omitempty"`
}

// ChunkType classifies a streaming response chunk.
type ChunkType string

const (
	// ChunkText carries a text fragment from the model.
	ChunkText ChunkType = "text"
	// ChunkError signals a terminal error; no further chunks follow.
	ChunkError ChunkType = "error"
	// ChunkDone signals the end of a successful stream.
	ChunkDone ChunkType = "done"
)

// Chunk is a single unit of a streaming response.
type Chunk struct {
	Type  ChunkType `json:"type"`
	Text  string    `json:"text,omitempty"`
	Error string    `json:"error,omitempty"`
}

// Conversation is the interface all provider components must implement.
// Chat begins a streaming inference call and returns a channel of Chunks.
// The channel is closed after a ChunkDone or ChunkError chunk is emitted.
// Implementations must stop and clean up when ctx is canceled.
type Conversation interface {
	Chat(ctx context.Context, req Request) (<-chan Chunk, error)
}
