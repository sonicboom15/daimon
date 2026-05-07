// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package session_test

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/sonicboom15/daimon/internal/conversation"
	"github.com/sonicboom15/daimon/internal/session"
)

// fakeStore is a minimal SessionStore for testing.
type fakeStore struct {
	data map[string][]conversation.Message
}

func newFakeStore() *fakeStore { return &fakeStore{data: map[string][]conversation.Message{}} }

func (f *fakeStore) Get(_ context.Context, id string) ([]conversation.Message, error) {
	msgs, ok := f.data[id]
	if !ok {
		return nil, nil
	}
	return msgs, nil
}

func (f *fakeStore) Set(_ context.Context, id string, msgs []conversation.Message) error {
	f.data[id] = msgs
	return nil
}

func (f *fakeStore) Delete(_ context.Context, id string) error {
	delete(f.data, id)
	return nil
}

func TestRegisterAndNew(t *testing.T) {
	const typeName = "test-session-register"
	session.Register(typeName, func(_ session.SessionConfig) (session.SessionStore, error) {
		return newFakeStore(), nil
	})

	st, err := session.New(typeName, session.SessionConfig{})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if st == nil {
		t.Fatal("New returned nil store")
	}
}

func TestNew_UnknownType(t *testing.T) {
	_, err := session.New("no-such-session-xyz", session.SessionConfig{})
	if err == nil {
		t.Fatal("expected error for unknown type, got nil")
	}
	if !strings.Contains(err.Error(), "no-such-session-xyz") {
		t.Errorf("error %q does not mention the unknown type", err.Error())
	}
}

func TestNew_ConfigPassthrough(t *testing.T) {
	const typeName = "test-session-config"
	var got session.SessionConfig
	session.Register(typeName, func(cfg session.SessionConfig) (session.SessionStore, error) {
		got = cfg
		return newFakeStore(), nil
	})

	want := session.SessionConfig{Metadata: map[string]string{"addr": "localhost:6379"}}
	if _, err := session.New(typeName, want); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Metadata["addr"] != "localhost:6379" {
		t.Errorf("metadata not passed through: got %v", got.Metadata)
	}
}

func TestRegister_Concurrent(t *testing.T) {
	var wg sync.WaitGroup
	for i := range 10 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			name := "concurrent-session-" + string(rune('a'+n))
			session.Register(name, func(_ session.SessionConfig) (session.SessionStore, error) {
				return newFakeStore(), nil
			})
		}(i)
	}
	wg.Wait()
}
