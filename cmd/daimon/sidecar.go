// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/sonicboom15/daimon/internal/config"
	"github.com/sonicboom15/daimon/internal/conversation"
	"github.com/sonicboom15/daimon/internal/server"
	"github.com/sonicboom15/daimon/internal/telemetry"
)

// buildSidecar loads config, wires components, and returns a ready-to-serve
// *http.Server plus a shutdown function that flushes telemetry.
// The caller is responsible for calling srv.ListenAndServe and shutdown.
func buildSidecar(configPath string) (srv *http.Server, shutdown func(context.Context) error, err error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, nil, fmt.Errorf("loading config: %w", err)
	}

	telShutdown, err := telemetry.Setup(context.Background(), cfg.Telemetry.OTLPEndpoint)
	if err != nil {
		return nil, nil, fmt.Errorf("setting up telemetry: %w", err)
	}

	components := make(map[string]conversation.Conversation, len(cfg.Components))
	for _, comp := range cfg.Components {
		compCfg := conversation.ComponentConfig{
			Metadata: comp.Metadata,
			Models:   make(map[string]conversation.ModelConfig, len(comp.Models)),
		}
		for model, mc := range comp.Models {
			compCfg.Models[model] = conversation.ModelConfig{APIKey: mc.APIKey}
		}
		c, err := conversation.New(comp.Type, compCfg)
		if err != nil {
			_ = telShutdown(context.Background())
			return nil, nil, fmt.Errorf("creating component %q: %w", comp.Name, err)
		}
		components[comp.Name] = c
		slog.Info("registered component", "name", comp.Name, "type", comp.Type)
	}

	srv = &http.Server{
		Addr:    fmt.Sprintf("127.0.0.1:%d", cfg.Port),
		Handler: server.New(components),
	}

	shutdown = func(ctx context.Context) error {
		defer telShutdown(context.Background()) //nolint:errcheck
		return srv.Shutdown(ctx)
	}

	return srv, shutdown, nil
}
