// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

// Package redis provides a Redis-backed SessionStore.
// Register type: "session/redis".
//
// Metadata keys:
//
//	addr     — Redis address, default localhost:6379
//	password — Redis password, default empty
//	db       — Redis DB index, default 0
//	ttl      — session TTL, e.g. "24h". Default "0" (no expiry)
package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/sonicboom15/daimon/internal/conversation"
	"github.com/sonicboom15/daimon/internal/session"
)

func init() {
	session.Register("session/redis", func(cfg session.SessionConfig) (session.SessionStore, error) {
		return New(cfg)
	})
}

// Store is a Redis-backed session store.
type Store struct {
	client *goredis.Client
	ttl    time.Duration
}

// New creates a Store from the provided config.
func New(cfg session.SessionConfig) (*Store, error) {
	meta := cfg.Metadata

	addr := meta["addr"]
	if addr == "" {
		addr = "localhost:6379"
	}

	db := 0
	if s := meta["db"]; s != "" {
		var err error
		db, err = strconv.Atoi(s)
		if err != nil {
			return nil, fmt.Errorf("session/redis: invalid db %q: %w", s, err)
		}
	}

	var ttl time.Duration
	if s := meta["ttl"]; s != "" && s != "0" {
		var err error
		ttl, err = time.ParseDuration(s)
		if err != nil {
			return nil, fmt.Errorf("session/redis: invalid ttl %q: %w", s, err)
		}
	}

	client := goredis.NewClient(&goredis.Options{
		Addr:     addr,
		Password: meta["password"],
		DB:       db,
	})

	return &Store{client: client, ttl: ttl}, nil
}

func key(id string) string { return "session:" + id }

func (s *Store) Get(ctx context.Context, id string) ([]conversation.Message, error) {
	val, err := s.client.Get(ctx, key(id)).Result()
	if err == goredis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("session/redis: GET: %w", err)
	}
	var msgs []conversation.Message
	if err := json.Unmarshal([]byte(val), &msgs); err != nil {
		return nil, fmt.Errorf("session/redis: unmarshal: %w", err)
	}
	return msgs, nil
}

func (s *Store) Set(ctx context.Context, id string, messages []conversation.Message) error {
	data, err := json.Marshal(messages)
	if err != nil {
		return fmt.Errorf("session/redis: marshal: %w", err)
	}
	if err := s.client.Set(ctx, key(id), data, s.ttl).Err(); err != nil {
		return fmt.Errorf("session/redis: SET: %w", err)
	}
	return nil
}

func (s *Store) Delete(ctx context.Context, id string) error {
	if err := s.client.Del(ctx, key(id)).Err(); err != nil {
		return fmt.Errorf("session/redis: DEL: %w", err)
	}
	return nil
}
