// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"github.com/sonicboom15/daimon/internal/conversation"
	"github.com/sonicboom15/daimon/internal/memory"
)

func (s *Server) handleConverse(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tracer := otel.Tracer("daimon/server")
	ctx, span := tracer.Start(ctx, "converse")
	defer span.End()

	componentName := r.PathValue("component")
	comp, ok := s.components[componentName]
	if !ok {
		http.Error(w, fmt.Sprintf("component %q not found", componentName), http.StatusNotFound)
		return
	}

	var req conversation.Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid request body: %s", err), http.StatusBadRequest)
		return
	}

	// Extract session ID and clear it so providers never see it.
	sessionID := req.SessionID
	req.SessionID = ""

	// Prepend stored history when the caller has an active session.
	if sessionID != "" {
		history, err := s.sessions.Get(ctx, sessionID)
		if err == nil && len(history) > 0 {
			req.Messages = append(history, req.Messages...)
		}
	}

	// RAG enrichment: pre-query the component's associated vector store with
	// the last user message and inject results as a leading system message.
	if storeName, ok := s.componentStores[componentName]; ok {
		if ms, ok := s.stores[storeName]; ok {
			if queryText := lastUserContent(req.Messages); queryText != "" {
				if results, err := ms.Query(ctx, queryText, 5); err == nil && len(results) > 0 {
					var sb strings.Builder
					sb.WriteString("Relevant context from memory:\n")
					for i, r := range results {
						fmt.Fprintf(&sb, "%d. %s\n", i+1, r.Content)
					}
					req.Messages = append(
						[]conversation.Message{{Role: conversation.RoleSystem, Content: sb.String()}},
						req.Messages...,
					)
				}
			}
		}
	}

	// Inject all tools (MCP + store + graph) so every request sees the full catalogue.
	req.Tools = append(req.Tools, s.tools...)

	span.SetAttributes(
		attribute.String("gen_ai.component", componentName),
		attribute.String("gen_ai.request.model", req.Model),
	)

	// First Chat call before committing to SSE, so we can still return HTTP errors.
	chunks, err := comp.Chat(ctx, req)
	if err != nil {
		http.Error(w, fmt.Sprintf("chat error: %s", err), http.StatusInternalServerError)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported by this server", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	writeSSE := func(chunk conversation.Chunk) {
		data, _ := json.Marshal(chunk)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}

	// Agentic loop: run Chat, collect tool calls, execute them, repeat until
	// the model returns pure text with no tool calls.
	var textBuf strings.Builder
	for {
		textBuf.Reset()
		var toolCalls []conversation.ToolCall

		for chunk := range chunks {
			switch chunk.Type {
			case conversation.ChunkText:
				writeSSE(chunk)
				textBuf.WriteString(chunk.Text)
			case conversation.ChunkToolCall:
				writeSSE(chunk)
				toolCalls = append(toolCalls, *chunk.ToolCall)
			case conversation.ChunkError:
				writeSSE(chunk)
				return
			case conversation.ChunkDone:
				if len(toolCalls) == 0 {
					if sessionID != "" {
						_ = s.sessions.Set(ctx, sessionID, append(req.Messages, conversation.Message{
							Role:    conversation.RoleAssistant,
							Content: textBuf.String(),
						}))
					}
					writeSSE(chunk)
					return
				}
			}
		}

		if len(toolCalls) == 0 {
			return
		}

		req.Messages = append(req.Messages, conversation.Message{
			Role:      conversation.RoleAssistant,
			ToolCalls: toolCalls,
		})

		for _, tc := range toolCalls {
			result, toolErr := s.executeTool(ctx, tc.Name, tc.Input)
			if toolErr != nil {
				result = fmt.Sprintf("error: %s", toolErr)
			}
			req.Messages = append(req.Messages, conversation.Message{
				Role:       conversation.RoleTool,
				Content:    result,
				ToolCallID: tc.ID,
			})
		}

		chunks, err = comp.Chat(ctx, req)
		if err != nil {
			writeSSE(conversation.Chunk{Type: conversation.ChunkError, Error: err.Error()})
			return
		}
	}
}

func (s *Server) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	_ = s.sessions.Delete(r.Context(), r.PathValue("id"))
	w.WriteHeader(http.StatusNoContent)
}

// executeTool dispatches a tool call to: vector store → graph store → MCP tool.
func (s *Server) executeTool(ctx context.Context, name string, input json.RawMessage) (string, error) {
	// Vector store tools.
	if ms, ok := s.storeRoutes[name]; ok {
		return s.executeStoreTool(ctx, name, ms, input)
	}
	// Graph store tools.
	if gs, ok := s.graphRoutes[name]; ok {
		return s.executeGraphTool(ctx, name, gs, input)
	}
	// MCP tools.
	caller, ok := s.toolRoutes[name]
	if !ok {
		return "", fmt.Errorf("unknown tool %q", name)
	}
	return caller.CallTool(ctx, name, input)
}

func (s *Server) executeStoreTool(ctx context.Context, name string, ms memory.MemoryStore, input json.RawMessage) (string, error) {
	var args map[string]any
	if err := json.Unmarshal(input, &args); err != nil {
		return "", fmt.Errorf("store tool %q: invalid input: %w", name, err)
	}

	if strings.HasSuffix(name, "_search") {
		query, _ := args["query"].(string)
		topK := 5
		if v, ok := args["top_k"].(float64); ok {
			topK = int(v)
		}
		results, err := ms.Query(ctx, query, topK)
		if err != nil {
			return "", fmt.Errorf("store search: %w", err)
		}
		b, _ := json.Marshal(map[string]any{"results": results})
		return string(b), nil
	}

	if strings.HasSuffix(name, "_upsert") {
		id, _ := args["id"].(string)
		content, _ := args["content"].(string)
		meta := map[string]string{}
		if m, ok := args["metadata"].(map[string]any); ok {
			for k, v := range m {
				if sv, ok := v.(string); ok {
					meta[k] = sv
				}
			}
		}
		assignedID, err := ms.Upsert(ctx, id, content, meta)
		if err != nil {
			return "", fmt.Errorf("store upsert: %w", err)
		}
		b, _ := json.Marshal(map[string]string{"id": assignedID})
		return string(b), nil
	}

	return "", fmt.Errorf("unknown store operation in tool %q", name)
}

func (s *Server) executeGraphTool(ctx context.Context, name string, gs memory.GraphStore, input json.RawMessage) (string, error) {
	var args map[string]any
	if err := json.Unmarshal(input, &args); err != nil {
		return "", fmt.Errorf("graph tool %q: invalid input: %w", name, err)
	}

	switch {
	case strings.HasSuffix(name, "_cypher"):
		query, _ := args["query"].(string)
		params, _ := args["params"].(map[string]any)
		rows, err := gs.Cypher(ctx, query, params)
		if err != nil {
			return "", fmt.Errorf("graph cypher: %w", err)
		}
		b, _ := json.Marshal(map[string]any{"rows": rows})
		return string(b), nil

	case strings.HasSuffix(name, "_add_node"):
		id, _ := args["id"].(string)
		var labels []string
		if ls, ok := args["labels"].([]any); ok {
			for _, l := range ls {
				if s, ok := l.(string); ok {
					labels = append(labels, s)
				}
			}
		}
		props, _ := args["props"].(map[string]any)
		assignedID, err := gs.AddNode(ctx, id, labels, props)
		if err != nil {
			return "", fmt.Errorf("graph add_node: %w", err)
		}
		b, _ := json.Marshal(map[string]string{"id": assignedID})
		return string(b), nil

	case strings.HasSuffix(name, "_add_edge"):
		from, _ := args["from"].(string)
		to, _ := args["to"].(string)
		relType, _ := args["type"].(string)
		props, _ := args["props"].(map[string]any)
		if err := gs.AddEdge(ctx, from, to, relType, props); err != nil {
			return "", fmt.Errorf("graph add_edge: %w", err)
		}
		return `{"ok":true}`, nil
	}

	return "", fmt.Errorf("unknown graph operation in tool %q", name)
}

// lastUserContent returns the Content of the last RoleUser message, or "".
func lastUserContent(msgs []conversation.Message) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == conversation.RoleUser {
			return msgs[i].Content
		}
	}
	return ""
}
