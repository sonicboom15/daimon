// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

// Package gemini provides a Conversation implementation backed by the native Google Gemini API.
package gemini

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/sonicboom15/daimon/internal/conversation"
)

func init() {
	conversation.Register("gemini", func(cfg conversation.ComponentConfig) (conversation.Conversation, error) {
		return New(cfg)
	})
}

// Component implements conversation.Conversation using the Gemini REST API.
type Component struct {
	apiKey       string
	baseURL      string
	defaultModel string
	defaults     conversation.ComponentDefaults
	httpClient   *http.Client
}

// New creates a Component from a ComponentConfig.
//
// Metadata keys:
//
//	api_key       — falls back to GEMINI_API_KEY then GOOGLE_API_KEY env vars
//	base_url      — defaults to https://generativelanguage.googleapis.com
//	default_model — defaults to gemini-2.0-flash
func New(cfg conversation.ComponentConfig) (*Component, error) {
	apiKey := cfg.Metadata["api_key"]
	if apiKey == "" {
		apiKey = os.Getenv("GEMINI_API_KEY")
	}
	if apiKey == "" {
		apiKey = os.Getenv("GOOGLE_API_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("gemini: api_key required in metadata or GEMINI_API_KEY / GOOGLE_API_KEY env var")
	}

	baseURL := cfg.Metadata["base_url"]
	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com"
	}

	defaultModel := cfg.Metadata["default_model"]
	if defaultModel == "" {
		defaultModel = cfg.Metadata["model"]
	}
	if defaultModel == "" {
		defaultModel = "gemini-2.0-flash"
	}

	return &Component{
		apiKey:       apiKey,
		baseURL:      baseURL,
		defaultModel: defaultModel,
		defaults:     cfg.Defaults,
		httpClient:   &http.Client{},
	}, nil
}

// ── Gemini wire types ────────────────────────────────────────────────────────

type geminiRequest struct {
	Contents          []geminiContent  `json:"contents"`
	SystemInstruction *geminiContent   `json:"systemInstruction,omitempty"`
	Tools             []geminiTool     `json:"tools,omitempty"`
	GenerationConfig  *geminiGenConfig `json:"generationConfig,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text             string            `json:"text,omitempty"`
	FunctionCall     *geminiFuncCall   `json:"functionCall,omitempty"`
	FunctionResponse *geminiFuncResp   `json:"functionResponse,omitempty"`
}

type geminiFuncCall struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
}

type geminiFuncResp struct {
	Name     string         `json:"name"`
	Response map[string]any `json:"response"`
}

type geminiTool struct {
	FunctionDeclarations []geminiFuncDecl `json:"functionDeclarations"`
}

type geminiFuncDecl struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type geminiGenConfig struct {
	Temperature     *float64 `json:"temperature,omitempty"`
	MaxOutputTokens *int     `json:"maxOutputTokens,omitempty"`
	TopP            *float64 `json:"topP,omitempty"`
	TopK            *int64   `json:"topK,omitempty"`
	StopSequences   []string `json:"stopSequences,omitempty"`
}

type geminiStreamEvent struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text         string `json:"text"`
				FunctionCall *struct {
					Name string         `json:"name"`
					Args map[string]any `json:"args"`
				} `json:"functionCall"`
			} `json:"parts"`
			Role string `json:"role"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// ── Chat ─────────────────────────────────────────────────────────────────────

// Chat implements conversation.Conversation.
func (c *Component) Chat(ctx context.Context, req conversation.Request) (<-chan conversation.Chunk, error) {
	model := req.Model
	if model == "" {
		model = c.defaultModel
	}

	sysInstr := buildSystemInstruction(effectiveSystem(c.defaults.System, req), req.Messages)
	contents, err := buildContents(req.Messages)
	if err != nil {
		return nil, err
	}

	greq := geminiRequest{
		Contents:          contents,
		SystemInstruction: sysInstr,
	}

	if len(req.Tools) > 0 {
		greq.Tools = buildTools(req.Tools)
	}

	genCfg := buildGenConfig(req, c.defaults)
	if genCfg != nil {
		greq.GenerationConfig = genCfg
	}

	body, err := json.Marshal(greq)
	if err != nil {
		return nil, fmt.Errorf("gemini: marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/v1beta/models/%s:streamGenerateContent?alt=sse", c.baseURL, model)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-goog-api-key", c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("gemini: request: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("gemini: HTTP %d: %s", resp.StatusCode, string(body))
	}

	ch := make(chan conversation.Chunk)
	go func() {
		defer close(ch)
		defer resp.Body.Close()

		type funcAcc struct {
			name string
			args map[string]any
		}
		var funcCalls []funcAcc

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			default:
			}

			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			jsonData := line[6:]

			var event geminiStreamEvent
			if err := json.Unmarshal([]byte(jsonData), &event); err != nil {
				continue
			}

			if event.Error != nil {
				select {
				case ch <- conversation.Chunk{Type: conversation.ChunkError, Error: event.Error.Message}:
				case <-ctx.Done():
				}
				return
			}

			for _, candidate := range event.Candidates {
				for _, part := range candidate.Content.Parts {
					if part.Text != "" {
						select {
						case ch <- conversation.Chunk{Type: conversation.ChunkText, Text: part.Text}:
						case <-ctx.Done():
							return
						}
					}
					if part.FunctionCall != nil {
						funcCalls = append(funcCalls, funcAcc{
							name: part.FunctionCall.Name,
							args: part.FunctionCall.Args,
						})
					}
				}
			}
		}

		if err := scanner.Err(); err != nil && ctx.Err() == nil {
			select {
			case ch <- conversation.Chunk{Type: conversation.ChunkError, Error: err.Error()}:
			case <-ctx.Done():
			}
			return
		}

		for i, fc := range funcCalls {
			args := fc.args
			if args == nil {
				args = map[string]any{}
			}
			input, _ := json.Marshal(args)
			select {
			case ch <- conversation.Chunk{
				Type: conversation.ChunkToolCall,
				ToolCall: &conversation.ToolCall{
					ID:    fmt.Sprintf("call_%d", i),
					Name:  fc.name,
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

// ── Builders ─────────────────────────────────────────────────────────────────

func buildSystemInstruction(sys string, msgs []conversation.Message) *geminiContent {
	var parts []geminiPart
	if sys != "" {
		parts = append(parts, geminiPart{Text: sys})
	}
	for _, m := range msgs {
		if m.Role == conversation.RoleSystem {
			parts = append(parts, geminiPart{Text: m.Content})
		}
	}
	if len(parts) == 0 {
		return nil
	}
	return &geminiContent{Parts: parts}
}

// buildContents converts daimon messages to Gemini contents, merging consecutive
// tool-result messages into a single user turn with multiple functionResponse parts.
func buildContents(msgs []conversation.Message) ([]geminiContent, error) {
	// Map tool_call_id → function_name from prior assistant tool-call messages.
	callIDToName := map[string]string{}
	for _, m := range msgs {
		if m.Role == conversation.RoleAssistant {
			for _, tc := range m.ToolCalls {
				callIDToName[tc.ID] = tc.Name
			}
		}
	}

	var contents []geminiContent
	var pendingToolParts []geminiPart

	flush := func() {
		if len(pendingToolParts) > 0 {
			contents = append(contents, geminiContent{Role: "user", Parts: pendingToolParts})
			pendingToolParts = nil
		}
	}

	for _, m := range msgs {
		switch m.Role {
		case conversation.RoleSystem:
			// handled via systemInstruction
		case conversation.RoleUser:
			flush()
			contents = append(contents, geminiContent{
				Role:  "user",
				Parts: []geminiPart{{Text: m.Content}},
			})
		case conversation.RoleAssistant:
			flush()
			if len(m.ToolCalls) > 0 {
				parts := make([]geminiPart, 0, len(m.ToolCalls))
				for _, tc := range m.ToolCalls {
					var args map[string]any
					if err := json.Unmarshal(tc.Input, &args); err != nil || args == nil {
						args = map[string]any{}
					}
					parts = append(parts, geminiPart{
						FunctionCall: &geminiFuncCall{Name: tc.Name, Args: args},
					})
				}
				contents = append(contents, geminiContent{Role: "model", Parts: parts})
			} else {
				contents = append(contents, geminiContent{
					Role:  "model",
					Parts: []geminiPart{{Text: m.Content}},
				})
			}
		case conversation.RoleTool:
			name := callIDToName[m.ToolCallID]
			if name == "" {
				name = m.ToolCallID
			}
			pendingToolParts = append(pendingToolParts, geminiPart{
				FunctionResponse: &geminiFuncResp{
					Name:     name,
					Response: map[string]any{"result": m.Content},
				},
			})
		}
	}
	flush()

	return contents, nil
}

func buildTools(tools []conversation.Tool) []geminiTool {
	decls := make([]geminiFuncDecl, 0, len(tools))
	for _, t := range tools {
		var params map[string]any
		_ = json.Unmarshal(t.InputSchema, &params)
		decls = append(decls, geminiFuncDecl{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  params,
		})
	}
	return []geminiTool{{FunctionDeclarations: decls}}
}

func buildGenConfig(req conversation.Request, defaults conversation.ComponentDefaults) *geminiGenConfig {
	cfg := &geminiGenConfig{}
	any := false
	if t := first(req.Temperature, defaults.Temperature); t != nil {
		cfg.Temperature = t
		any = true
	}
	if n := firstInt(req.MaxTokens, defaults.MaxTokens); n > 0 {
		cfg.MaxOutputTokens = &n
		any = true
	}
	if p := first(req.TopP, defaults.TopP); p != nil {
		cfg.TopP = p
		any = true
	}
	if k := first(req.TopK, defaults.TopK); k != nil {
		cfg.TopK = k
		any = true
	}
	if stop := firstSlice(req.Stop, defaults.Stop); len(stop) > 0 {
		cfg.StopSequences = stop
		any = true
	}
	if !any {
		return nil
	}
	return cfg
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
