// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"encoding/json"
	"fmt"
	"net/http"

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

	span.SetAttributes(
		attribute.String("gen_ai.component", componentName),
		attribute.String("gen_ai.request.model", req.Model),
	)

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported by this server", http.StatusInternalServerError)
		return
	}

	chunks, err := comp.Chat(ctx, req)
	if err != nil {
		http.Error(w, fmt.Sprintf("chat error: %s", err), http.StatusInternalServerError)
		return
	}

	// All validation passed — commit to an SSE response.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	for chunk := range chunks {
		data, _ := json.Marshal(chunk)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
		if chunk.Type == conversation.ChunkDone || chunk.Type == conversation.ChunkError {
			return
		}
	}
}
