// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"context"
	"sync"

	"github.com/sonicboom15/daimon/internal/conversation"
)

// InMemory is a thread-safe in-memory SessionStore.
// History is lost when the process restarts.
// It is the default when no session component is configured.
type InMemory struct {
	mu       sync.RWMutex
	sessions map[string][]conversation.Message
}

// NewInMemory creates a ready-to-use in-memory session store.
func NewInMemory() *InMemory {
	return &InMemory{sessions: make(map[string][]conversation.Message)}
}

func (s *InMemory) Get(_ context.Context, id string) ([]conversation.Message, error) {
	s.mu.RLock()
	msgs, ok := s.sessions[id]
	s.mu.RUnlock()
	if !ok {
		return nil, nil
	}
	out := make([]conversation.Message, len(msgs))
	copy(out, msgs)
	return out, nil
}

func (s *InMemory) Set(_ context.Context, id string, messages []conversation.Message) error {
	out := make([]conversation.Message, len(messages))
	copy(out, messages)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[id] = out
	return nil
}

func (s *InMemory) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, id)
	return nil
}

// Len returns the number of active sessions (useful in tests).
func (s *InMemory) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.sessions)
}

func init() {
	Register("session/inmemory", func(_ SessionConfig) (SessionStore, error) {
		return NewInMemory(), nil
	})
}
