// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

// Package server implements the daimon HTTP API.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/sonicboom15/daimon/internal/conversation"
	"github.com/sonicboom15/daimon/internal/mcp"
	"github.com/sonicboom15/daimon/internal/memory"
	"github.com/sonicboom15/daimon/internal/session"
)

// toolCaller is the subset of mcp.Client used by the handler.
// Defined as an interface so the handler can be tested without a real subprocess.
type toolCaller interface {
	CallTool(ctx context.Context, name string, input json.RawMessage) (string, error)
}

// Server routes HTTP requests to provider components and drives the MCP
// agentic loop when tools are available.
type Server struct {
	mux             *http.ServeMux
	components      map[string]conversation.Conversation
	stores          map[string]memory.MemoryStore
	graphs          map[string]memory.GraphStore
	componentStores map[string]string // LLM component name → vector store name (RAG)
	tools           []conversation.Tool   // aggregated from MCP servers + auto-generated store tools
	toolRoutes      map[string]toolCaller // MCP tool name → owning MCP client
	storeRoutes     map[string]memory.MemoryStore // store tool name prefix → store
	graphRoutes     map[string]memory.GraphStore  // graph tool name prefix → graph store
	sessions        session.SessionStore
}

// New creates a Server, pre-fetches tool catalogues from all MCP clients,
// generates store/graph tool definitions, and registers HTTP routes.
func New(
	components map[string]conversation.Conversation,
	mcpClients []*mcp.Client,
	stores map[string]memory.MemoryStore,
	graphs map[string]memory.GraphStore,
	componentStores map[string]string,
	sessionSt session.SessionStore,
) *Server {
	s := &Server{
		mux:             http.NewServeMux(),
		components:      components,
		stores:          stores,
		graphs:          graphs,
		componentStores: componentStores,
		toolRoutes:      make(map[string]toolCaller),
		storeRoutes:     make(map[string]memory.MemoryStore),
		graphRoutes:     make(map[string]memory.GraphStore),
		sessions:        sessionSt,
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

	// Auto-generate tool definitions for every vector store.
	for name, ms := range stores {
		safeName := strings.ReplaceAll(name, "-", "_")
		searchTool := conversation.Tool{
			Name:        safeName + "_search",
			Description: fmt.Sprintf("Search %s for documents semantically similar to a query.", name),
			InputSchema: json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"},"top_k":{"type":"integer","default":5}},"required":["query"]}`),
		}
		upsertTool := conversation.Tool{
			Name:        safeName + "_upsert",
			Description: fmt.Sprintf("Store a document in %s.", name),
			InputSchema: json.RawMessage(`{"type":"object","properties":{"id":{"type":"string"},"content":{"type":"string"},"metadata":{"type":"object"}},"required":["content"]}`),
		}
		s.tools = append(s.tools, searchTool, upsertTool)
		s.storeRoutes[safeName+"_search"] = ms
		s.storeRoutes[safeName+"_upsert"] = ms
		slog.Info("registered vector store tools", "store", name)
	}

	// Auto-generate tool definitions for every graph store.
	for name, gs := range graphs {
		safeName := strings.ReplaceAll(name, "-", "_")
		cypherTool := conversation.Tool{
			Name:        safeName + "_cypher",
			Description: fmt.Sprintf("Run a Cypher query against the %s graph database.", name),
			InputSchema: json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"},"params":{"type":"object"}},"required":["query"]}`),
		}
		addNodeTool := conversation.Tool{
			Name:        safeName + "_add_node",
			Description: fmt.Sprintf("Add or update a node in the %s graph database.", name),
			InputSchema: json.RawMessage(`{"type":"object","properties":{"id":{"type":"string"},"labels":{"type":"array","items":{"type":"string"}},"props":{"type":"object"}},"required":[]}`),
		}
		addEdgeTool := conversation.Tool{
			Name:        safeName + "_add_edge",
			Description: fmt.Sprintf("Create a directed relationship between two nodes in %s.", name),
			InputSchema: json.RawMessage(`{"type":"object","properties":{"from":{"type":"string"},"to":{"type":"string"},"type":{"type":"string"},"props":{"type":"object"}},"required":["from","to","type"]}`),
		}
		s.tools = append(s.tools, cypherTool, addNodeTool, addEdgeTool)
		s.graphRoutes[safeName+"_cypher"] = gs
		s.graphRoutes[safeName+"_add_node"] = gs
		s.graphRoutes[safeName+"_add_edge"] = gs
		slog.Info("registered graph store tools", "store", name)
	}

	s.routes()
	return s
}

func (s *Server) routes() {
	s.mux.HandleFunc("POST /v1/converse/{component}", s.handleConverse)
	s.mux.HandleFunc("DELETE /v1/sessions/{id}", s.handleDeleteSession)
	s.mux.HandleFunc("GET /healthz", s.handleHealthz)

	// Vector store CRUD.
	s.mux.HandleFunc("PUT /v1/memory/{store}/{id}", s.handleMemoryUpsertWithID)
	s.mux.HandleFunc("POST /v1/memory/{store}", s.handleMemoryUpsert)
	s.mux.HandleFunc("POST /v1/memory/{store}/query", s.handleMemoryQuery)
	s.mux.HandleFunc("DELETE /v1/memory/{store}/{id}", s.handleMemoryDelete)

	// Graph store operations.
	s.mux.HandleFunc("PUT /v1/graph/{store}/nodes/{id}", s.handleGraphAddNodeWithID)
	s.mux.HandleFunc("POST /v1/graph/{store}/nodes", s.handleGraphAddNode)
	s.mux.HandleFunc("POST /v1/graph/{store}/edges", s.handleGraphAddEdge)
	s.mux.HandleFunc("POST /v1/graph/{store}/cypher", s.handleGraphCypher)
	s.mux.HandleFunc("DELETE /v1/graph/{store}/nodes/{id}", s.handleGraphDelete)
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}
