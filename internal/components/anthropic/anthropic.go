// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

// Package anthropic provides a Conversation implementation backed by the Anthropic API.
package anthropic

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	aparam "github.com/anthropics/anthropic-sdk-go/packages/param"

	"github.com/sonicboom15/daimon/internal/conversation"
)

func init() {
	conversation.Register("anthropic", func(metadata map[string]string) (conversation.Conversation, error) {
		return New(metadata)
	})
}

// Component implements conversation.Conversation using the Anthropic Messages API.
type Component struct {
	client       anthropic.Client
	defaultModel string
}

// New creates a Component from metadata.
// Recognized keys: api_key (falls back to ANTHROPIC_API_KEY), model (default: claude-opus-4-7).
func New(metadata map[string]string) (*Component, error) {
	apiKey := metadata["api_key"]
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("anthropic: api_key required in metadata or ANTHROPIC_API_KEY env var")
	}
	model := metadata["model"]
	if model == "" {
		model = "claude-opus-4-7"
	}
	return &Component{
		client:       anthropic.NewClient(option.WithAPIKey(apiKey)),
		defaultModel: model,
	}, nil
}

// Chat implements conversation.Conversation.
func (c *Component) Chat(ctx context.Context, req conversation.Request) (<-chan conversation.Chunk, error) {
	model := req.Model
	if model == "" {
		model = c.defaultModel
	}

	// The Anthropic API separates system prompts from the messages array.
	var systemParts []string
	var msgs []anthropic.MessageParam
	for _, m := range req.Messages {
		switch m.Role {
		case conversation.RoleSystem:
			systemParts = append(systemParts, m.Content)
		case conversation.RoleAssistant:
			msgs = append(msgs, anthropic.NewAssistantMessage(anthropic.NewTextBlock(m.Content)))
		default:
			msgs = append(msgs, anthropic.NewUserMessage(anthropic.NewTextBlock(m.Content)))
		}
	}

	maxTokens := int64(req.MaxTokens)
	if maxTokens == 0 {
		maxTokens = 4096
	}

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: maxTokens,
		Messages:  msgs,
	}
	if len(systemParts) > 0 {
		params.System = []anthropic.TextBlockParam{
			{Text: strings.Join(systemParts, "\n")},
		}
	}
	if req.Temperature != nil {
		params.Temperature = aparam.NewOpt(*req.Temperature)
	}

	stream := c.client.Messages.NewStreaming(ctx, params)

	ch := make(chan conversation.Chunk)
	go func() {
		defer close(ch)

		for stream.Next() {
			event := stream.Current()
			if event.Type == "content_block_delta" &&
				event.Delta.Type == "text_delta" &&
				event.Delta.Text != "" {
				select {
				case ch <- conversation.Chunk{Type: conversation.ChunkText, Text: event.Delta.Text}:
				case <-ctx.Done():
					return
				}
			}
		}
		if err := stream.Err(); err != nil {
			select {
			case ch <- conversation.Chunk{Type: conversation.ChunkError, Error: err.Error()}:
			case <-ctx.Done():
			}
			return
		}
		select {
		case ch <- conversation.Chunk{Type: conversation.ChunkDone}:
		case <-ctx.Done():
		}
	}()

	return ch, nil
}
