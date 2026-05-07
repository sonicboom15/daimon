// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

// Package postgres provides a PostgreSQL-backed SessionStore.
// Register type: "session/postgres".
//
// Metadata keys:
//
//	dsn   — PostgreSQL DSN (required), e.g. postgres://user:pass@localhost:5432/mydb
//	table — table name, default daimon_sessions
//	ttl   — session TTL, e.g. "24h". Default "" (no expiry)
package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sonicboom15/daimon/internal/conversation"
	"github.com/sonicboom15/daimon/internal/session"
)

func init() {
	session.Register("session/postgres", func(cfg session.SessionConfig) (session.SessionStore, error) {
		return New(cfg)
	})
}

// Store is a Postgres-backed session store.
type Store struct {
	pool  *pgxpool.Pool
	table string
	ttl   time.Duration
}

// New creates a Store and initialises the sessions table.
func New(cfg session.SessionConfig) (*Store, error) {
	meta := cfg.Metadata

	dsn := meta["dsn"]
	if dsn == "" {
		return nil, fmt.Errorf("session/postgres: dsn is required")
	}

	table := meta["table"]
	if table == "" {
		table = "daimon_sessions"
	}

	var ttl time.Duration
	if s := meta["ttl"]; s != "" {
		var err error
		ttl, err = time.ParseDuration(s)
		if err != nil {
			return nil, fmt.Errorf("session/postgres: invalid ttl %q: %w", s, err)
		}
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		return nil, fmt.Errorf("session/postgres: connect: %w", err)
	}

	st := &Store{pool: pool, table: table, ttl: ttl}
	if err := st.migrate(context.Background()); err != nil {
		pool.Close()
		return nil, fmt.Errorf("session/postgres: migrate: %w", err)
	}
	return st, nil
}

func (s *Store) migrate(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id         TEXT PRIMARY KEY,
			messages   JSONB NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`, s.table))
	return err
}

func (s *Store) Get(ctx context.Context, id string) ([]conversation.Message, error) {
	q := fmt.Sprintf(`SELECT messages FROM %s WHERE id = $1`, s.table)
	if s.ttl > 0 {
		q = fmt.Sprintf(`SELECT messages FROM %s WHERE id = $1 AND updated_at > now() - interval '%d seconds'`,
			s.table, int(s.ttl.Seconds()))
	}
	var raw []byte
	err := s.pool.QueryRow(ctx, q, id).Scan(&raw)
	if err != nil {
		// pgx returns pgx.ErrNoRows as an error; treat it as empty session.
		return nil, nil //nolint:nilerr
	}
	var msgs []conversation.Message
	if err := json.Unmarshal(raw, &msgs); err != nil {
		return nil, fmt.Errorf("session/postgres: unmarshal: %w", err)
	}
	return msgs, nil
}

func (s *Store) Set(ctx context.Context, id string, messages []conversation.Message) error {
	data, err := json.Marshal(messages)
	if err != nil {
		return fmt.Errorf("session/postgres: marshal: %w", err)
	}
	_, err = s.pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s (id, messages, updated_at)
		VALUES ($1, $2, now())
		ON CONFLICT (id) DO UPDATE SET messages = $2, updated_at = now()`,
		s.table), id, data)
	if err != nil {
		return fmt.Errorf("session/postgres: upsert: %w", err)
	}
	return nil
}

func (s *Store) Delete(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, fmt.Sprintf(`DELETE FROM %s WHERE id = $1`, s.table), id)
	if err != nil {
		return fmt.Errorf("session/postgres: delete: %w", err)
	}
	return nil
}
