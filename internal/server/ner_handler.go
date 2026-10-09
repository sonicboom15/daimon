// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/sonicboom15/daimon/internal/ner"
)

func (s *Server) handleNERExtract(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	nm, ok := s.nerModels[name]
	if !ok {
		http.Error(w, fmt.Sprintf("ner model %q not found", name), http.StatusNotFound)
		return
	}

	var req ner.ExtractRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid request body: %s", err), http.StatusBadRequest)
		return
	}

	resp, err := nm.Extract(r.Context(), req)
	if err != nil {
		http.Error(w, fmt.Sprintf("ner extract error: %s", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

