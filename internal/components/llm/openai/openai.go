// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

// Package openai provides a Conversation implementation backed by the OpenAI API.
package openai

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
	conversation.Register("openai", func(cfg conversation.ComponentConfig) (conversation.Conversation, error) {
		return New(cfg)
	})
}

// Component implements conversation.Conversation using the OpenAI chat completions API.
type Component struct {
	defaultClient openai.Client
	defaultModel  string
	modelClients  map[string]openai.Client
	defaults      conversation.ComponentDefaults
}

// New creates a Component from a ComponentConfig.
// Metadata keys: api_key (falls back to OPENAI_API_KEY), default_model (falls back to model, then gpt-4o).
func New(cfg conversation.ComponentConfig) (*Component, error) {
	defaultKey := cfg.Metadata["api_key"]
	if defaultKey == "" {
		defaultKey = os.Getenv("OPENAI_API_KEY")
	}
	if defaultKey == "" {
		return nil, fmt.Errorf("openai: api_key required in metadata or OPENAI_API_KEY env var")
	}

	defaultModel := cfg.Metadata["default_model"]
	if defaultModel == "" {
		defaultModel = cfg.Metadata["model"]
	}
	if defaultModel == "" {
		defaultModel = "gpt-4o"
	}

	modelClients := make(map[string]openai.Client, len(cfg.Models))
	for model, mc := range cfg.Models {
		key := mc.APIKey
		if key == "" {
			key = defaultKey
		}
		modelClients[model] = openai.NewClient(option.WithAPIKey(key))
	}

	return &Component{
		defaultClient: openai.NewClient(option.WithAPIKey(defaultKey)),
		defaultModel:  defaultModel,
		modelClients:  modelClients,
		defaults:      cfg.Defaults,
	}, nil
}

func (c *Component) clientFor(model string) openai.Client {
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

	msgs := buildMessages(req.Messages, effectiveSystem(c.defaults.System, req))

	params := openai.ChatCompletionNewParams{
		Model:    openai.ChatModel(model),
		Messages: msgs,
	}

	// Sampling: request overrides component default; zero/nil means "not set".
	if t := first(req.Temperature, c.defaults.Temperature); t != nil {
		params.Temperature = oaiparam.NewOpt(*t)
	}
	if n := firstInt(req.MaxTokens, c.defaults.MaxTokens); n > 0 {
		params.MaxTokens = oaiparam.NewOpt(int64(n))
	}
	if p := first(req.TopP, c.defaults.TopP); p != nil {
		params.TopP = oaiparam.NewOpt(*p)
	}
	if f := first(req.FrequencyPenalty, c.defaults.FrequencyPenalty); f != nil {
		params.FrequencyPenalty = oaiparam.NewOpt(*f)
	}
	if p := first(req.PresencePenalty, c.defaults.PresencePenalty); p != nil {
		params.PresencePenalty = oaiparam.NewOpt(*p)
	}
	if s := first(req.Seed, c.defaults.Seed); s != nil {
		params.Seed = oaiparam.NewOpt(*s)
	}
	if stop := firstSlice(req.Stop, c.defaults.Stop); len(stop) > 0 {
		params.Stop = openai.ChatCompletionNewParamsStopUnion{OfStringArray: stop}
	}
	if len(req.Tools) > 0 {
		params.Tools = buildTools(req.Tools)
	}

	client := c.clientFor(model)
	stream := client.Chat.Completions.NewStreaming(ctx, params)

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

// buildMessages converts conversation messages to OpenAI params, prepending
// system if provided.
func buildMessages(msgs []conversation.Message, system string) []openai.ChatCompletionMessageParamUnion {
	out := make([]openai.ChatCompletionMessageParamUnion, 0, len(msgs)+1)
	if system != "" {
		out = append(out, openai.SystemMessage(system))
	}
	for _, m := range msgs {
		switch m.Role {
		case conversation.RoleSystem:
			if system == "" { // only include if we didn't inject one above
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

// effectiveSystem returns the system prompt to use: req.System wins, then
// defaults.System (only if no system message already in req.Messages).
func effectiveSystem(defaultSys string, req conversation.Request) string {
	if req.System != "" {
		return req.System
	}
	if defaultSys != "" {
		for _, m := range req.Messages {
			if m.Role == conversation.RoleSystem {
				return "" // caller already provided a system message
			}
		}
		return defaultSys
	}
	return ""
}

// first returns the first non-nil pointer from a list.
func first[T any](vals ...*T) *T {
	for _, v := range vals {
		if v != nil {
			return v
		}
	}
	return nil
}

// firstInt returns the first non-zero int.
func firstInt(vals ...int) int {
	for _, v := range vals {
		if v != 0 {
			return v
		}
	}
	return 0
}

// firstSlice returns the first non-empty slice.
func firstSlice[T any](vals ...[]T) []T {
	for _, v := range vals {
		if len(v) > 0 {
			return v
		}
	}
	return nil
}
