// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"sync"

	"github.com/sonicboom15/daimon/internal/conversation"
)

// sessionStore is a thread-safe in-memory store for conversation histories.
// Each session maps a caller-provided string ID to the full message history
// accumulated across all turns. History is lost when the process restarts.
type sessionStore struct {
	mu       sync.RWMutex
	sessions map[string][]conversation.Message
}

func newSessionStore() *sessionStore {
	return &sessionStore{sessions: make(map[string][]conversation.Message)}
}

// get returns a copy of the stored message history for id.
// Returns (nil, false) if no session with that id exists.
func (s *sessionStore) get(id string) ([]conversation.Message, bool) {
	s.mu.RLock()
	msgs, ok := s.sessions[id]
	s.mu.RUnlock()
	if !ok {
		return nil, false
	}
	out := make([]conversation.Message, len(msgs))
	copy(out, msgs)
	return out, true
}

// set stores a copy of messages under id, replacing any existing history.
func (s *sessionStore) set(id string, messages []conversation.Message) {
	out := make([]conversation.Message, len(messages))
	copy(out, messages)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[id] = out
}

// delete removes the session for id. No-op if id is not found.
func (s *sessionStore) delete(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, id)
}

// len returns the number of active sessions.
func (s *sessionStore) len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.sessions)
}
