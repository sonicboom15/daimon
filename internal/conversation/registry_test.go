// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package conversation_test

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/sonicboom15/daimon/internal/conversation"
)

// fakeConversation is a minimal Conversation implementation for testing.
type fakeConversation struct{}

func (f *fakeConversation) Chat(_ context.Context, _ conversation.Request) (<-chan conversation.Chunk, error) {
	ch := make(chan conversation.Chunk, 1)
	ch <- conversation.Chunk{Type: conversation.ChunkDone}
	close(ch)
	return ch, nil
}

func TestRegisterAndNew(t *testing.T) {
	const typeName = "test-fake-" + "register"
	conversation.Register(typeName, func(_ conversation.ComponentConfig) (conversation.Conversation, error) {
		return &fakeConversation{}, nil
	})

	conv, err := conversation.New(typeName, conversation.ComponentConfig{})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if conv == nil {
		t.Fatal("New returned nil conversation")
	}
}

func TestNew_UnknownType(t *testing.T) {
	_, err := conversation.New("no-such-type-xyz", conversation.ComponentConfig{})
	if err == nil {
		t.Fatal("expected error for unknown type, got nil")
	}
	if !strings.Contains(err.Error(), "no-such-type-xyz") {
		t.Errorf("error %q does not mention the unknown type", err.Error())
	}
}

func TestNew_ConfigPassthrough(t *testing.T) {
	const typeName = "test-fake-config"
	var got conversation.ComponentConfig
	conversation.Register(typeName, func(cfg conversation.ComponentConfig) (conversation.Conversation, error) {
		got = cfg
		return &fakeConversation{}, nil
	})

	want := conversation.ComponentConfig{
		Metadata: map[string]string{"api_key": "secret"},
	}
	if _, err := conversation.New(typeName, want); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Metadata["api_key"] != "secret" {
		t.Errorf("metadata not passed through: got %v", got.Metadata)
	}
}

func TestRegister_Concurrent(t *testing.T) {
	// Verify that concurrent Register calls don't race (run with -race).
	var wg sync.WaitGroup
	for i := range 10 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			name := "concurrent-fake-" + string(rune('a'+n))
			conversation.Register(name, func(_ conversation.ComponentConfig) (conversation.Conversation, error) {
				return &fakeConversation{}, nil
			})
		}(i)
	}
	wg.Wait()
}
