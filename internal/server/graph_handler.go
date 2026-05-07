// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func (s *Server) resolveGraph(w http.ResponseWriter, r *http.Request) (storeName string, ok bool) {
	storeName = r.PathValue("store")
	if _, exists := s.graphs[storeName]; !exists {
		http.Error(w, fmt.Sprintf("graph store %q not found", storeName), http.StatusNotFound)
		return "", false
	}
	return storeName, true
}

func (s *Server) handleGraphAddNodeWithID(w http.ResponseWriter, r *http.Request) {
	storeName, ok := s.resolveGraph(w, r)
	if !ok {
		return
	}
	id := r.PathValue("id")
	s.doAddNode(w, r, storeName, id)
}

func (s *Server) handleGraphAddNode(w http.ResponseWriter, r *http.Request) {
	storeName, ok := s.resolveGraph(w, r)
	if !ok {
		return
	}
	s.doAddNode(w, r, storeName, "")
}

type addNodeBody struct {
	Labels []string       `json:"labels"`
	Props  map[string]any `json:"props"`
}

func (s *Server) doAddNode(w http.ResponseWriter, r *http.Request, storeName, id string) {
	var body addNodeBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, fmt.Sprintf("invalid request body: %s", err), http.StatusBadRequest)
		return
	}
	assignedID, err := s.graphs[storeName].AddNode(r.Context(), id, body.Labels, body.Props)
	if err != nil {
		http.Error(w, fmt.Sprintf("add node failed: %s", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"id": assignedID})
}

func (s *Server) handleGraphAddEdge(w http.ResponseWriter, r *http.Request) {
	storeName, ok := s.resolveGraph(w, r)
	if !ok {
		return
	}
	var body struct {
		From  string         `json:"from"`
		To    string         `json:"to"`
		Type  string         `json:"type"`
		Props map[string]any `json:"props"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, fmt.Sprintf("invalid request body: %s", err), http.StatusBadRequest)
		return
	}
	if err := s.graphs[storeName].AddEdge(r.Context(), body.From, body.To, body.Type, body.Props); err != nil {
		http.Error(w, fmt.Sprintf("add edge failed: %s", err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleGraphCypher(w http.ResponseWriter, r *http.Request) {
	storeName, ok := s.resolveGraph(w, r)
	if !ok {
		return
	}
	var body struct {
		Query  string         `json:"query"`
		Params map[string]any `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, fmt.Sprintf("invalid request body: %s", err), http.StatusBadRequest)
		return
	}
	rows, err := s.graphs[storeName].Cypher(r.Context(), body.Query, body.Params)
	if err != nil {
		http.Error(w, fmt.Sprintf("cypher failed: %s", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"rows": rows})
}

func (s *Server) handleGraphDelete(w http.ResponseWriter, r *http.Request) {
	storeName, ok := s.resolveGraph(w, r)
	if !ok {
		return
	}
	id := r.PathValue("id")
	if err := s.graphs[storeName].Delete(r.Context(), id); err != nil {
		http.Error(w, fmt.Sprintf("delete failed: %s", err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
