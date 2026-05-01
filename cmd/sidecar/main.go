// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

// Command sidecar is the daimon sidecar process.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/sonicboom15/daimon/internal/config"
	"github.com/sonicboom15/daimon/internal/conversation"
	"github.com/sonicboom15/daimon/internal/server"
	"github.com/sonicboom15/daimon/internal/telemetry"

	// Import components for side-effect registration.
	_ "github.com/sonicboom15/daimon/internal/components/anthropic"
	_ "github.com/sonicboom15/daimon/internal/components/openai"
)

func main() {
	configPath := flag.String("config", "examples/config.yaml", "path to YAML config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("loading config", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	shutdown, err := telemetry.Setup(ctx, cfg.Telemetry.OTLPEndpoint)
	if err != nil {
		slog.Error("setting up telemetry", "err", err)
		os.Exit(1)
	}
	defer shutdown(context.Background()) //nolint:errcheck

	components := make(map[string]conversation.Conversation, len(cfg.Components))
	for _, comp := range cfg.Components {
		c, err := conversation.New(comp.Type, comp.Metadata)
		if err != nil {
			slog.Error("creating component", "name", comp.Name, "type", comp.Type, "err", err)
			os.Exit(1)
		}
		components[comp.Name] = c
		slog.Info("registered component", "name", comp.Name, "type", comp.Type)
	}

	srv := &http.Server{
		Addr:    fmt.Sprintf("127.0.0.1:%d", cfg.Port),
		Handler: server.New(components),
	}

	go func() {
		<-ctx.Done()
		slog.Info("shutting down")
		if err := srv.Shutdown(context.Background()); err != nil {
			slog.Error("shutdown error", "err", err)
		}
	}()

	slog.Info("daimon listening", "addr", srv.Addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}
