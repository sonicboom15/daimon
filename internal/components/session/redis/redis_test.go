// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package redis_test

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"

	"github.com/sonicboom15/daimon/internal/conversation"
	"github.com/sonicboom15/daimon/internal/session"
	_ "github.com/sonicboom15/daimon/internal/components/session/redis"
)

func newStore(t *testing.T) (session.SessionStore, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	st, err := session.New("session/redis", session.SessionConfig{
		Metadata: map[string]string{"addr": mr.Addr()},
	})
	if err != nil {
		t.Fatalf("session.New: %v", err)
	}
	return st, mr
}

func TestRedis_SetAndGet(t *testing.T) {
	st, _ := newStore(t)
	msgs := []conversation.Message{
		{Role: conversation.RoleUser, Content: "hello"},
		{Role: conversation.RoleAssistant, Content: "hi"},
	}
	if err := st.Set(context.Background(), "s1", msgs); err != nil {
		t.Fatalf("Set: %v", err)
	}

	got, err := st.Get(context.Background(), "s1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Content != "hello" {
		t.Errorf("got[0].Content = %q, want hello", got[0].Content)
	}
	if got[1].Role != conversation.RoleAssistant {
		t.Errorf("got[1].Role = %q, want assistant", got[1].Role)
	}
}

func TestRedis_GetMissing(t *testing.T) {
	st, _ := newStore(t)
	msgs, err := st.Get(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if msgs != nil {
		t.Errorf("expected nil for missing key, got %v", msgs)
	}
}

func TestRedis_Delete(t *testing.T) {
	st, _ := newStore(t)
	_ = st.Set(context.Background(), "s1", []conversation.Message{{Role: conversation.RoleUser, Content: "hi"}})
	if err := st.Delete(context.Background(), "s1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	msgs, _ := st.Get(context.Background(), "s1")
	if msgs != nil {
		t.Errorf("expected nil after delete, got %v", msgs)
	}
}

func TestRedis_DeleteNoOp(t *testing.T) {
	st, _ := newStore(t)
	if err := st.Delete(context.Background(), "nonexistent"); err != nil {
		t.Errorf("Delete of nonexistent key returned error: %v", err)
	}
}

func TestRedis_OverwriteOnSet(t *testing.T) {
	st, _ := newStore(t)
	_ = st.Set(context.Background(), "s1", []conversation.Message{{Role: conversation.RoleUser, Content: "v1"}})
	_ = st.Set(context.Background(), "s1", []conversation.Message{{Role: conversation.RoleUser, Content: "v2"}})

	got, _ := st.Get(context.Background(), "s1")
	if len(got) != 1 || got[0].Content != "v2" {
		t.Errorf("Set did not overwrite: got %v", got)
	}
}
