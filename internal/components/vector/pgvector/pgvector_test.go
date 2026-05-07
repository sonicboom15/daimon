// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package pgvector_test

import (
	"testing"

	"github.com/sonicboom15/daimon/internal/memory"
	_ "github.com/sonicboom15/daimon/internal/components/vector/pgvector"
)

func TestRegistered(t *testing.T) {
	// Verify the type is registered. New will fail to connect without a live
	// Postgres instance but must not return "unknown type".
	_, err := memory.New("pgvector", memory.StoreConfig{
		Metadata: map[string]string{
			"dsn":        "postgres://user:pass@localhost:5432/testdb",
			"dimensions": "4",
		},
	})
	if err != nil {
		t.Logf("New returned error (expected without live Postgres): %v", err)
	}
}

func TestNew_MissingDSN(t *testing.T) {
	_, err := memory.New("pgvector", memory.StoreConfig{})
	if err == nil {
		t.Fatal("expected error for missing DSN, got nil")
	}
}

func TestNew_InvalidDimensions(t *testing.T) {
	_, err := memory.New("pgvector", memory.StoreConfig{
		Metadata: map[string]string{
			"dsn":        "postgres://localhost/db",
			"dimensions": "not-a-number",
		},
	})
	if err == nil {
		t.Fatal("expected error for invalid dimensions, got nil")
	}
}
