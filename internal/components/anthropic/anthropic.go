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
	conversation.Register("anthropic", func(cfg conversation.ComponentConfig) (conversation.Conversation, error) {
		return New(cfg)
	})
}

// Component implements conversation.Conversation using the Anthropic Messages API.
type Component struct {
	defaultClient anthropic.Client
	defaultModel  string
	// modelClients holds a pre-built client for each model that has its own API key.
	modelClients map[string]anthropic.Client
}

// New creates a Component from a ComponentConfig.
// Metadata keys: api_key (falls back to ANTHROPIC_API_KEY), default_model (falls back to model, then claude-opus-4-7).
// Per-model overrides live in cfg.Models[modelName].APIKey.
func New(cfg conversation.ComponentConfig) (*Component, error) {
	defaultKey := cfg.Metadata["api_key"]
	if defaultKey == "" {
		defaultKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	if defaultKey == "" {
		return nil, fmt.Errorf("anthropic: api_key required in metadata or ANTHROPIC_API_KEY env var")
	}

	defaultModel := cfg.Metadata["default_model"]
	if defaultModel == "" {
		defaultModel = cfg.Metadata["model"] // backward compat
	}
	if defaultModel == "" {
		defaultModel = "claude-opus-4-7"
	}

	modelClients := make(map[string]anthropic.Client, len(cfg.Models))
	for model, mc := range cfg.Models {
		key := mc.APIKey
		if key == "" {
			key = defaultKey
		}
		modelClients[model] = anthropic.NewClient(option.WithAPIKey(key))
	}

	return &Component{
		defaultClient: anthropic.NewClient(option.WithAPIKey(defaultKey)),
		defaultModel:  defaultModel,
		modelClients:  modelClients,
	}, nil
}

func (c *Component) clientFor(model string) anthropic.Client {
	if cl, ok := c.modelClients[model]; ok {
		return cl
	}
	return c.defaultClient
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

	client := c.clientFor(model)
	stream := client.Messages.NewStreaming(ctx, params)

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
