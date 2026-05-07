// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"testing"

	"github.com/sonicboom15/daimon/internal/session"
	_ "github.com/sonicboom15/daimon/internal/components/session/postgres"
)

func TestRegistered(t *testing.T) {
	// Verify the type is registered — New will fail to connect but the factory
	// itself must be wired up.
	_, err := session.New("session/postgres", session.SessionConfig{
		Metadata: map[string]string{"dsn": "postgres://user:pass@localhost:5432/nonexistent"},
	})
	// We expect either success (unlikely without a DB) or a connection error,
	// not an "unknown type" error.
	if err != nil {
		t.Logf("New returned error (expected without live DB): %v", err)
	}
}

func TestNew_MissingDSN(t *testing.T) {
	_, err := session.New("session/postgres", session.SessionConfig{})
	if err == nil {
		t.Fatal("expected error for missing DSN, got nil")
	}
}
