// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

// Package mistral provides a Conversation implementation backed by the Mistral AI API.
// Mistral exposes an OpenAI-compatible endpoint, so this provider reuses the
// openai-go client (no new dependency) with a Mistral-specific base URL and API key.
package mistral

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	oaiparam "github.com/openai/openai-go/packages/param"

	"github.com/sonicboom15/daimon/internal/conversation"
)

func init() {
	conversation.Register("mistral", func(cfg conversation.ComponentConfig) (conversation.Conversation, error) {
		return New(cfg)
	})
}

// Component implements conversation.Conversation using the Mistral AI API.
type Component struct {
	client       openai.Client
	defaultModel string
	defaults     conversation.ComponentDefaults
}

// New creates a Component from a ComponentConfig.
//
// Metadata keys:
//
//	api_key       — falls back to MISTRAL_API_KEY env var
//	base_url      — defaults to https://api.mistral.ai/v1
//	default_model — defaults to mistral-large-latest
func New(cfg conversation.ComponentConfig) (*Component, error) {
	apiKey := cfg.Metadata["api_key"]
	if apiKey == "" {
		apiKey = os.Getenv("MISTRAL_API_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("mistral: api_key required in metadata or MISTRAL_API_KEY env var")
	}

	baseURL := cfg.Metadata["base_url"]
	if baseURL == "" {
		baseURL = "https://api.mistral.ai/v1"
	}

	defaultModel := cfg.Metadata["default_model"]
	if defaultModel == "" {
		defaultModel = cfg.Metadata["model"]
	}
	if defaultModel == "" {
		defaultModel = "mistral-large-latest"
	}

	return &Component{
		client:       openai.NewClient(option.WithBaseURL(baseURL), option.WithAPIKey(apiKey)),
		defaultModel: defaultModel,
		defaults:     cfg.Defaults,
	}, nil
}

// Chat implements conversation.Conversation.
func (c *Component) Chat(ctx context.Context, req conversation.Request) (<-chan conversation.Chunk, error) {
	model := req.Model
	if model == "" {
		model = c.defaultModel
	}

	msgs := buildMessages(req.Messages, effectiveSystem(c.defaults.System, req))

	params := openai.ChatCompletionNewParams{
		Model:    openai.ChatModel(model),
		Messages: msgs,
	}

	if t := first(req.Temperature, c.defaults.Temperature); t != nil {
		params.Temperature = oaiparam.NewOpt(*t)
	}
	if n := firstInt(req.MaxTokens, c.defaults.MaxTokens); n > 0 {
		params.MaxTokens = oaiparam.NewOpt(int64(n))
	}
	if p := first(req.TopP, c.defaults.TopP); p != nil {
		params.TopP = oaiparam.NewOpt(*p)
	}
	if stop := firstSlice(req.Stop, c.defaults.Stop); len(stop) > 0 {
		params.Stop = openai.ChatCompletionNewParamsStopUnion{OfStringArray: stop}
	}
	if len(req.Tools) > 0 {
		params.Tools = buildTools(req.Tools)
	}

	stream := c.client.Chat.Completions.NewStreaming(ctx, params)

	ch := make(chan conversation.Chunk)
	go func() {
		defer close(ch)
		defer stream.Close()

		type toolAcc struct {
			id   string
			name string
			args strings.Builder
		}
		var accs []toolAcc

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
				for _, tc := range choice.Delta.ToolCalls {
					idx := int(tc.Index)
					for len(accs) <= idx {
						accs = append(accs, toolAcc{})
					}
					if tc.ID != "" {
						accs[idx].id = tc.ID
					}
					if tc.Function.Name != "" {
						accs[idx].name = tc.Function.Name
					}
					accs[idx].args.WriteString(tc.Function.Arguments)
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

		for _, acc := range accs {
			if acc.name == "" {
				continue
			}
			input := acc.args.String()
			if input == "" {
				input = "{}"
			}
			select {
			case ch <- conversation.Chunk{
				Type: conversation.ChunkToolCall,
				ToolCall: &conversation.ToolCall{
					ID:    acc.id,
					Name:  acc.name,
					Input: json.RawMessage(input),
				},
			}:
			case <-ctx.Done():
				return
			}
		}

		select {
		case ch <- conversation.Chunk{Type: conversation.ChunkDone}:
		case <-ctx.Done():
		}
	}()

	return ch, nil
}

func buildMessages(msgs []conversation.Message, system string) []openai.ChatCompletionMessageParamUnion {
	out := make([]openai.ChatCompletionMessageParamUnion, 0, len(msgs)+1)
	if system != "" {
		out = append(out, openai.SystemMessage(system))
	}
	for _, m := range msgs {
		switch m.Role {
		case conversation.RoleSystem:
			if system == "" {
				out = append(out, openai.SystemMessage(m.Content))
			}
		case conversation.RoleAssistant:
			if len(m.ToolCalls) > 0 {
				tcs := make([]openai.ChatCompletionMessageToolCallParam, len(m.ToolCalls))
				for i, tc := range m.ToolCalls {
					tcs[i] = openai.ChatCompletionMessageToolCallParam{
						ID:   tc.ID,
						Type: "function",
						Function: openai.ChatCompletionMessageToolCallFunctionParam{
							Name:      tc.Name,
							Arguments: string(tc.Input),
						},
					}
				}
				out = append(out, openai.ChatCompletionMessageParamUnion{
					OfAssistant: &openai.ChatCompletionAssistantMessageParam{ToolCalls: tcs},
				})
			} else {
				out = append(out, openai.AssistantMessage(m.Content))
			}
		case conversation.RoleTool:
			out = append(out, openai.ToolMessage(m.Content, m.ToolCallID))
		default:
			out = append(out, openai.UserMessage(m.Content))
		}
	}
	return out
}

func buildTools(tools []conversation.Tool) []openai.ChatCompletionToolParam {
	out := make([]openai.ChatCompletionToolParam, 0, len(tools))
	for _, t := range tools {
		var schema map[string]interface{}
		_ = json.Unmarshal(t.InputSchema, &schema)
		if schema == nil {
			schema = map[string]interface{}{"type": "object"}
		}
		out = append(out, openai.ChatCompletionToolParam{
			Function: openai.FunctionDefinitionParam{
				Name:        t.Name,
				Description: oaiparam.NewOpt(t.Description),
				Parameters:  openai.FunctionParameters(schema),
			},
		})
	}
	return out
}

func effectiveSystem(defaultSys string, req conversation.Request) string {
	if req.System != "" {
		return req.System
	}
	if defaultSys != "" {
		for _, m := range req.Messages {
			if m.Role == conversation.RoleSystem {
				return ""
			}
		}
		return defaultSys
	}
	return ""
}

func first[T any](vals ...*T) *T {
	for _, v := range vals {
		if v != nil {
			return v
		}
	}
	return nil
}

func firstInt(vals ...int) int {
	for _, v := range vals {
		if v != 0 {
			return v
		}
	}
	return 0
}

func firstSlice[T any](vals ...[]T) []T {
	for _, v := range vals {
		if len(v) > 0 {
			return v
		}
	}
	return nil
}
