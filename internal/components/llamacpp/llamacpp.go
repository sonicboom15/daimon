// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

// Package llamacpp provides a Conversation implementation for any server that
// exposes an OpenAI-compatible HTTP API, including llama.cpp and LM Studio.
package llamacpp

import (
	"context"
	"fmt"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	oaiparam "github.com/openai/openai-go/packages/param"

	"github.com/sonicboom15/daimon/internal/conversation"
)

func init() {
	conversation.Register("llamacpp", func(cfg conversation.ComponentConfig) (conversation.Conversation, error) {
		return New(cfg)
	})
}

// Component implements conversation.Conversation against any OpenAI-compatible
// local inference server (llama.cpp, LM Studio, Ollama, etc.).
type Component struct {
	client       openai.Client
	defaultModel string
}

// New creates a Component from a ComponentConfig.
//
// Metadata keys:
//
//	base_url      — full base URL including /v1 path (default: http://localhost:8080/v1)
//	default_model — model name sent to the server; many local servers ignore this
//	api_key       — passed as Bearer token; most local servers don't require one
//
// LM Studio default base_url: http://localhost:1234/v1
// llama.cpp default base_url: http://localhost:8080/v1
func New(cfg conversation.ComponentConfig) (*Component, error) {
	baseURL := cfg.Metadata["base_url"]
	if baseURL == "" {
		baseURL = "http://localhost:8080/v1"
	}

	// Local servers typically don't enforce auth, but the SDK requires a non-empty key.
	apiKey := cfg.Metadata["api_key"]
	if apiKey == "" {
		apiKey = "local"
	}

	defaultModel := cfg.Metadata["default_model"]
	if defaultModel == "" {
		defaultModel = cfg.Metadata["model"] // backward compat
	}
	if defaultModel == "" {
		return nil, fmt.Errorf("llamacpp: default_model is required (set it to the model name loaded in your server)")
	}

	return &Component{
		client:       openai.NewClient(option.WithBaseURL(baseURL), option.WithAPIKey(apiKey)),
		defaultModel: defaultModel,
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
