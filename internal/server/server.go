// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

// Package server implements the daimon HTTP API.
package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/sonicboom15/daimon/internal/conversation"
	"github.com/sonicboom15/daimon/internal/mcp"
)

// toolCaller is the subset of mcp.Client used by the handler.
// Defined as an interface so the handler can be tested without a real subprocess.
type toolCaller interface {
	CallTool(ctx context.Context, name string, input json.RawMessage) (string, error)
}

// Server routes HTTP requests to provider components and drives the MCP
// agentic loop when tools are available.
type Server struct {
	mux        *http.ServeMux
	components map[string]conversation.Conversation
	tools      []conversation.Tool      // aggregated from all MCP servers at startup
	toolRoutes map[string]toolCaller    // tool name → owning MCP client
}

// New creates a Server, pre-fetches tool catalogues from all MCP clients,
// and registers HTTP routes.
func New(components map[string]conversation.Conversation, mcpClients []*mcp.Client) *Server {
	s := &Server{
		mux:        http.NewServeMux(),
		components: components,
		toolRoutes: make(map[string]toolCaller),
	}

	ctx := context.Background()
	for _, client := range mcpClients {
		tools, err := client.ListTools(ctx)
		if err != nil {
			slog.Warn("could not list MCP tools", "server", client.Name(), "err", err)
			continue
		}
		for _, t := range tools {
			s.tools = append(s.tools, t)
			s.toolRoutes[t.Name] = client
			slog.Info("registered MCP tool", "server", client.Name(), "tool", t.Name)
		}
	}

	s.routes()
	return s
}

func (s *Server) routes() {
	s.mux.HandleFunc("POST /v1/converse/{component}", s.handleConverse)
	s.mux.HandleFunc("GET /healthz", s.handleHealthz)
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}
