// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/sonicboom15/daimon/internal/decision"
)

func (s *Server) handleDecisionChoose(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	dm, ok := s.decisionModels[name]
	if !ok {
		http.Error(w, fmt.Sprintf("decision model %q not found", name), http.StatusNotFound)
		return
	}

	var req decision.ChoiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid request body: %s", err), http.StatusBadRequest)
		return
	}

	resp, err := dm.Choose(r.Context(), req)
	if err != nil {
		http.Error(w, fmt.Sprintf("decision choose error: %s", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleDecisionVerify(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	dm, ok := s.decisionModels[name]
	if !ok {
		http.Error(w, fmt.Sprintf("decision model %q not found", name), http.StatusNotFound)
		return
	}

	var req decision.VerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid request body: %s", err), http.StatusBadRequest)
		return
	}

	resp, err := dm.Verify(r.Context(), req)
	if err != nil {
		http.Error(w, fmt.Sprintf("decision verify error: %s", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

