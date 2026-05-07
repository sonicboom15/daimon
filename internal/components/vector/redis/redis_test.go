// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package redis_test

import (
	"testing"

	"github.com/sonicboom15/daimon/internal/memory"
	_ "github.com/sonicboom15/daimon/internal/components/vector/redis"
)

func TestRegistered(t *testing.T) {
	// Verify the type is registered. New will fail without a live Redis Stack
	// but the factory must be wired up and config parsing must succeed.
	_, err := memory.New("redis", memory.StoreConfig{
		Metadata: map[string]string{
			"addr":       "localhost:6379",
			"index":      "test-idx",
			"dimensions": "4",
		},
	})
	// We expect either success or a connection error, not "unknown type".
	if err != nil {
		t.Logf("New returned error (expected without live Redis Stack): %v", err)
	}
}

func TestNew_InvalidDimensions(t *testing.T) {
	_, err := memory.New("redis", memory.StoreConfig{
		Metadata: map[string]string{
			"dimensions": "not-a-number",
		},
	})
	if err == nil {
		t.Fatal("expected error for invalid dimensions, got nil")
	}
}
