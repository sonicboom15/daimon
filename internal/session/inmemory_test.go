// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package session_test

import (
	"context"
	"sync"
	"testing"

	"github.com/sonicboom15/daimon/internal/conversation"
	"github.com/sonicboom15/daimon/internal/session"
)

func TestInMemory_GetMissing(t *testing.T) {
	s := session.NewInMemory()
	msgs, err := s.Get(context.Background(), "missing")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if msgs != nil {
		t.Errorf("expected nil for missing key, got %v", msgs)
	}
}

func TestInMemory_SetAndGet(t *testing.T) {
	s := session.NewInMemory()
	in := []conversation.Message{
		{Role: conversation.RoleUser, Content: "hello"},
		{Role: conversation.RoleAssistant, Content: "hi"},
	}
	if err := s.Set(context.Background(), "s1", in); err != nil {
		t.Fatalf("Set: %v", err)
	}

	out, err := s.Get(context.Background(), "s1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("len = %d, want 2", len(out))
	}
	if out[0].Content != "hello" {
		t.Errorf("out[0].Content = %q, want hello", out[0].Content)
	}
}

func TestInMemory_GetReturnsCopy(t *testing.T) {
	s := session.NewInMemory()
	in := []conversation.Message{{Role: conversation.RoleUser, Content: "original"}}
	_ = s.Set(context.Background(), "s1", in)

	out, _ := s.Get(context.Background(), "s1")
	out[0].Content = "mutated"

	out2, _ := s.Get(context.Background(), "s1")
	if out2[0].Content != "original" {
		t.Errorf("stored value was mutated via returned slice; got %q", out2[0].Content)
	}
}

func TestInMemory_Delete(t *testing.T) {
	s := session.NewInMemory()
	_ = s.Set(context.Background(), "s1", []conversation.Message{{Role: conversation.RoleUser, Content: "hi"}})
	_ = s.Delete(context.Background(), "s1")

	msgs, _ := s.Get(context.Background(), "s1")
	if msgs != nil {
		t.Errorf("expected nil after delete, got %v", msgs)
	}
	if s.Len() != 0 {
		t.Errorf("Len() = %d, want 0", s.Len())
	}
}

func TestInMemory_DeleteNoOp(t *testing.T) {
	s := session.NewInMemory()
	if err := s.Delete(context.Background(), "nonexistent"); err != nil {
		t.Errorf("Delete of nonexistent key returned error: %v", err)
	}
}

func TestInMemory_Concurrent(t *testing.T) {
	s := session.NewInMemory()
	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			id := "session-" + string(rune('a'+n%26))
			_ = s.Set(context.Background(), id, []conversation.Message{{Role: conversation.RoleUser, Content: "msg"}})
			_, _ = s.Get(context.Background(), id)
			_ = s.Delete(context.Background(), id)
		}(i)
	}
	wg.Wait()
}

func TestInMemoryRegistered(t *testing.T) {
	st, err := session.New("session/inmemory", session.SessionConfig{})
	if err != nil {
		t.Fatalf("session/inmemory not registered: %v", err)
	}
	if st == nil {
		t.Fatal("New returned nil")
	}
}
