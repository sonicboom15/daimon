// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

// Package server implements the daimon HTTP API.
package server

import (
	"net/http"

	"github.com/sonicboom15/daimon/internal/conversation"
)

// Server routes HTTP requests to provider components.
type Server struct {
	mux        *http.ServeMux
	components map[string]conversation.Conversation
}

// New creates a Server with the given named components and registers routes.
func New(components map[string]conversation.Conversation) *Server {
	s := &Server{
		mux:        http.NewServeMux(),
		components: components,
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
