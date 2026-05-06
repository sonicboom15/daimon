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
		if history, ok := s.sessions.get(sessionID); ok {
			req.Messages = append(history, req.Messages...)
		}
	}

	// Inject MCP tools so every request sees the full tool catalogue.
	// req is decoded fresh from JSON each call, so this append is safe.
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
		data, _ := json.Marshal(chunk) // Chunk fields are all JSON-safe; error is impossible.
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}

	// Agentic loop: run Chat, collect tool calls, execute them via MCP,
	// append results, repeat until the model returns pure text.
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
				// Forward so clients can display progress ("calling tool X…").
				writeSSE(chunk)
				toolCalls = append(toolCalls, *chunk.ToolCall)
			case conversation.ChunkError:
				writeSSE(chunk)
				return
			case conversation.ChunkDone:
				if len(toolCalls) == 0 {
					// Final text-only pass — persist the completed exchange.
					if sessionID != "" {
						s.sessions.set(sessionID, append(req.Messages, conversation.Message{
							Role:    conversation.RoleAssistant,
							Content: textBuf.String(),
						}))
					}
					writeSSE(chunk)
					return
				}
				// Don't emit done — we're looping after tool execution.
			}
		}

		if len(toolCalls) == 0 {
			return
		}

		// Append the assistant's tool-call turn to the conversation.
		req.Messages = append(req.Messages, conversation.Message{
			Role:      conversation.RoleAssistant,
			ToolCalls: toolCalls,
		})

		// Execute each tool and append its result.
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

		// Next iteration with updated message history.
		chunks, err = comp.Chat(ctx, req)
		if err != nil {
			writeSSE(conversation.Chunk{Type: conversation.ChunkError, Error: err.Error()})
			return
		}
	}
}

func (s *Server) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	s.sessions.delete(r.PathValue("id"))
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) executeTool(ctx context.Context, name string, input json.RawMessage) (string, error) {
	client, ok := s.toolRoutes[name]
	if !ok {
		return "", fmt.Errorf("unknown tool %q", name)
	}
	return client.CallTool(ctx, name, input)
}
