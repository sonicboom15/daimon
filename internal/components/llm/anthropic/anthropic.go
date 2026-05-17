// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

// Package anthropic provides a Conversation implementation backed by the Anthropic API.
package anthropic

import (
	"context"
	"encoding/json"
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
	modelClients  map[string]anthropic.Client
	defaults      conversation.ComponentDefaults
}

// New creates a Component from a ComponentConfig.
// Metadata keys: api_key (falls back to ANTHROPIC_API_KEY), default_model (falls back to model, then claude-opus-4-7).
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
		defaultModel = cfg.Metadata["model"]
	}
	if defaultModel == "" {
		defaultModel = "claude-opus-4-7"
	}

	baseURL := cfg.Metadata["base_url"]
	makeClient := func(key string) anthropic.Client {
		opts := []option.RequestOption{option.WithAPIKey(key)}
		if baseURL != "" {
			opts = append(opts, option.WithBaseURL(baseURL))
		}
		return anthropic.NewClient(opts...)
	}

	modelClients := make(map[string]anthropic.Client, len(cfg.Models))
	for model, mc := range cfg.Models {
		key := mc.APIKey
		if key == "" {
			key = defaultKey
		}
		modelClients[model] = makeClient(key)
	}

	return &Component{
		defaultClient: makeClient(defaultKey),
		defaultModel:  defaultModel,
		modelClients:  modelClients,
		defaults:      cfg.Defaults,
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

	sys := effectiveSystem(c.defaults.System, req)
	msgs, err := buildMessages(req.Messages)
	if err != nil {
		return nil, err
	}

	maxTokens := firstInt(req.MaxTokens, c.defaults.MaxTokens, 4096)

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: int64(maxTokens),
		Messages:  msgs,
	}

	// Combine system from messages + effective system string.
	var systemParts []string
	for _, m := range req.Messages {
		if m.Role == conversation.RoleSystem {
			systemParts = append(systemParts, m.Content)
		}
	}
	if sys != "" {
		systemParts = append([]string{sys}, systemParts...)
	}
	if len(systemParts) > 0 {
		params.System = []anthropic.TextBlockParam{{Text: strings.Join(systemParts, "\n")}}
	}

	if t := first(req.Temperature, c.defaults.Temperature); t != nil {
		params.Temperature = aparam.NewOpt(*t)
	}
	if p := first(req.TopP, c.defaults.TopP); p != nil {
		params.TopP = aparam.NewOpt(*p)
	}
	if k := first(req.TopK, c.defaults.TopK); k != nil {
		params.TopK = aparam.NewOpt(*k)
	}
	if stop := firstSlice(req.Stop, c.defaults.Stop); len(stop) > 0 {
		params.StopSequences = stop
	}
	if len(req.Tools) > 0 {
		params.Tools = buildTools(req.Tools)
	}

	client := c.clientFor(model)
	stream := client.Messages.NewStreaming(ctx, params)

	ch := make(chan conversation.Chunk)
	go func() {
		defer close(ch)

		type toolAcc struct {
			id   string
			name string
			args strings.Builder
		}
		toolAccs := map[int64]*toolAcc{}
		var toolOrder []int64

		for stream.Next() {
			event := stream.Current()

			switch event.Type {
			case "content_block_start":
				if event.ContentBlock.Type == "tool_use" {
					toolAccs[event.Index] = &toolAcc{id: event.ContentBlock.ID, name: event.ContentBlock.Name}
					toolOrder = append(toolOrder, event.Index)
				}
			case "content_block_delta":
				switch event.Delta.Type {
				case "text_delta":
					if event.Delta.Text != "" {
						select {
						case ch <- conversation.Chunk{Type: conversation.ChunkText, Text: event.Delta.Text}:
						case <-ctx.Done():
							return
						}
					}
				case "input_json_delta":
					if acc, ok := toolAccs[event.Index]; ok {
						acc.args.WriteString(event.Delta.PartialJSON)
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

		for _, idx := range toolOrder {
			acc := toolAccs[idx]
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

func buildMessages(msgs []conversation.Message) ([]anthropic.MessageParam, error) {
	var out []anthropic.MessageParam
	for _, m := range msgs {
		switch m.Role {
		case conversation.RoleSystem:
			continue // handled via params.System
		case conversation.RoleAssistant:
			if len(m.ToolCalls) > 0 {
				blocks := make([]anthropic.ContentBlockParamUnion, len(m.ToolCalls))
				for i, tc := range m.ToolCalls {
					var input any
					_ = json.Unmarshal(tc.Input, &input)
					if input == nil {
						input = map[string]any{}
					}
					blocks[i] = anthropic.NewToolUseBlock(tc.ID, input, tc.Name)
				}
				out = append(out, anthropic.NewAssistantMessage(blocks...))
			} else {
				out = append(out, anthropic.NewAssistantMessage(anthropic.NewTextBlock(m.Content)))
			}
		case conversation.RoleTool:
			out = append(out, anthropic.NewUserMessage(
				anthropic.NewToolResultBlock(m.ToolCallID, m.Content, false),
			))
		default:
			out = append(out, anthropic.NewUserMessage(anthropic.NewTextBlock(m.Content)))
		}
	}
	return out, nil
}

func buildTools(tools []conversation.Tool) []anthropic.ToolUnionParam {
	out := make([]anthropic.ToolUnionParam, 0, len(tools))
	for _, t := range tools {
		var schema map[string]any
		_ = json.Unmarshal(t.InputSchema, &schema)

		inputSchema := anthropic.ToolInputSchemaParam{}
		if schema != nil {
			if props, ok := schema["properties"]; ok {
				inputSchema.Properties = props
			}
			if raw, ok := schema["required"].([]any); ok {
				required := make([]string, 0, len(raw))
				for _, r := range raw {
					if s, ok := r.(string); ok {
						required = append(required, s)
					}
				}
				inputSchema.Required = required
			}
		}
		tp := anthropic.ToolParam{
			Name:        t.Name,
			Description: aparam.NewOpt(t.Description),
			InputSchema: inputSchema,
		}
		out = append(out, anthropic.ToolUnionParam{OfTool: &tp})
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
