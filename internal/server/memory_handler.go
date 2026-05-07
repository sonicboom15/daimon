// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func (s *Server) resolveStore(w http.ResponseWriter, r *http.Request) (storeName string, ok bool) {
	storeName = r.PathValue("store")
	if _, exists := s.stores[storeName]; !exists {
		http.Error(w, fmt.Sprintf("vector store %q not found", storeName), http.StatusNotFound)
		return "", false
	}
	return storeName, true
}

func (s *Server) handleMemoryUpsertWithID(w http.ResponseWriter, r *http.Request) {
	storeName, ok := s.resolveStore(w, r)
	if !ok {
		return
	}
	id := r.PathValue("id")
	s.doUpsert(w, r, storeName, id)
}

func (s *Server) handleMemoryUpsert(w http.ResponseWriter, r *http.Request) {
	storeName, ok := s.resolveStore(w, r)
	if !ok {
		return
	}
	s.doUpsert(w, r, storeName, "")
}

type upsertBody struct {
	Content  string            `json:"content"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

func (s *Server) doUpsert(w http.ResponseWriter, r *http.Request, storeName, id string) {
	var body upsertBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, fmt.Sprintf("invalid request body: %s", err), http.StatusBadRequest)
		return
	}
	assignedID, err := s.stores[storeName].Upsert(r.Context(), id, body.Content, body.Metadata)
	if err != nil {
		http.Error(w, fmt.Sprintf("upsert failed: %s", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"id": assignedID})
}

func (s *Server) handleMemoryQuery(w http.ResponseWriter, r *http.Request) {
	storeName, ok := s.resolveStore(w, r)
	if !ok {
		return
	}
	var body struct {
		Query string `json:"query"`
		TopK  int    `json:"top_k"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, fmt.Sprintf("invalid request body: %s", err), http.StatusBadRequest)
		return
	}
	if body.TopK <= 0 {
		body.TopK = 5
	}
	results, err := s.stores[storeName].Query(r.Context(), body.Query, body.TopK)
	if err != nil {
		http.Error(w, fmt.Sprintf("query failed: %s", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"results": results})
}

func (s *Server) handleMemoryDelete(w http.ResponseWriter, r *http.Request) {
	storeName, ok := s.resolveStore(w, r)
	if !ok {
		return
	}
	id := r.PathValue("id")
	if err := s.stores[storeName].Delete(r.Context(), id); err != nil {
		http.Error(w, fmt.Sprintf("delete failed: %s", err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
