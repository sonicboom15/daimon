// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

// Package session defines the SessionStore interface and factory registry for
// persistent conversation history. The default in-memory implementation is
// always available; Redis and Postgres backends are opt-in components.
package session

import (
	"context"

	"github.com/sonicboom15/daimon/internal/conversation"
)

// SessionConfig is handed to every SessionFactory at construction time.
type SessionConfig struct {
	Metadata map[string]string
}

// SessionStore persists per-session conversation history.
// All methods must be safe for concurrent use.
type SessionStore interface {
	// Get returns the stored message history for id, or nil if no session exists.
	Get(ctx context.Context, id string) ([]conversation.Message, error)
	// Set stores messages under id, replacing any existing history.
	Set(ctx context.Context, id string, messages []conversation.Message) error
	// Delete removes the session for id. No-op if id is not found.
	Delete(ctx context.Context, id string) error
}

// SessionFactory creates a SessionStore from a SessionConfig.
type SessionFactory func(cfg SessionConfig) (SessionStore, error)
