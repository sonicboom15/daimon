// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

// Package openai provides a Conversation implementation backed by the OpenAI API.
package openai

import (
	"context"
	"fmt"
	"os"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	oaiparam "github.com/openai/openai-go/packages/param"

	"github.com/sonicboom15/daimon/internal/conversation"
)

func init() {
	conversation.Register("openai", func(metadata map[string]string) (conversation.Conversation, error) {
		return New(metadata)
	})
}

// Component implements conversation.Conversation using the OpenAI chat completions API.
type Component struct {
	client       openai.Client
	defaultModel string
}

// New creates a Component from metadata.
// Recognized keys: api_key (falls back to OPENAI_API_KEY), model (default: gpt-4o).
func New(metadata map[string]string) (*Component, error) {
	apiKey := metadata["api_key"]
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("openai: api_key required in metadata or OPENAI_API_KEY env var")
	}
	model := metadata["model"]
	if model == "" {
		model = "gpt-4o"
	}
	return &Component{
		client:       openai.NewClient(option.WithAPIKey(apiKey)),
		defaultModel: model,
	}, nil
}

// Chat implements conversation.Conversation.
func (c *Component) Chat(ctx context.Context, req conversation.Request) (<-chan conversation.Chunk, error) {
	model := req.Model
	if model == "" {
		model = c.defaultModel
	}

	msgs := make([]openai.ChatCompletionMessageParamUnion, 0, len(req.Messages))
	for _, m := range req.Messages {
		switch m.Role {
		case conversation.RoleSystem:
			msgs = append(msgs, openai.SystemMessage(m.Content))
		case conversation.RoleAssistant:
			msgs = append(msgs, openai.AssistantMessage(m.Content))
		default:
			msgs = append(msgs, openai.UserMessage(m.Content))
		}
	}

	params := openai.ChatCompletionNewParams{
		Model:    openai.ChatModel(model),
		Messages: msgs,
	}
	if req.MaxTokens > 0 {
		params.MaxTokens = oaiparam.NewOpt(int64(req.MaxTokens))
	}
	if req.Temperature != nil {
		params.Temperature = oaiparam.NewOpt(*req.Temperature)
	}

	stream := c.client.Chat.Completions.NewStreaming(ctx, params)

	ch := make(chan conversation.Chunk)
	go func() {
		defer close(ch)
		defer stream.Close()

		for stream.Next() {
			chunk := stream.Current()
			for _, choice := range chunk.Choices {
				if choice.Delta.Content != "" {
					select {
					case ch <- conversation.Chunk{Type: conversation.ChunkText, Text: choice.Delta.Content}:
					case <-ctx.Done():
						return
					}
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
