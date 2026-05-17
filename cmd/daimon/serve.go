// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	// LLM providers.
	_ "github.com/sonicboom15/daimon/internal/components/llm/anthropic"
	_ "github.com/sonicboom15/daimon/internal/components/llm/gemini"
	_ "github.com/sonicboom15/daimon/internal/components/llm/llamacpp"
	_ "github.com/sonicboom15/daimon/internal/components/llm/mistral"
	_ "github.com/sonicboom15/daimon/internal/components/llm/openai"

	// Embedding components.
	_ "github.com/sonicboom15/daimon/internal/components/embedding/openai"

	// Session store components.
	_ "github.com/sonicboom15/daimon/internal/components/session/postgres"
	_ "github.com/sonicboom15/daimon/internal/components/session/redis"

	// Vector store components.
	_ "github.com/sonicboom15/daimon/internal/components/vector/chroma"
	_ "github.com/sonicboom15/daimon/internal/components/vector/inmemory"
	_ "github.com/sonicboom15/daimon/internal/components/vector/pgvector"
	_ "github.com/sonicboom15/daimon/internal/components/vector/qdrant"
	_ "github.com/sonicboom15/daimon/internal/components/vector/redis"

	// Graph store components.
	_ "github.com/sonicboom15/daimon/internal/components/graph/memgraph"
	_ "github.com/sonicboom15/daimon/internal/components/graph/neo4j"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the daimon sidecar HTTP server",
	RunE:  runServe,
}

func runServe(cmd *cobra.Command, _ []string) error {
	configPath, _ := cmd.Flags().GetString("config")

	srv, shutdown, err := buildSidecar(configPath)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		slog.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdown(shutdownCtx)
	}()

	slog.Info("daimon listening", "addr", srv.Addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
