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
	"github.com/sonicboom15/daimon/internal/mcp"
	"github.com/sonicboom15/daimon/internal/server"
	"github.com/sonicboom15/daimon/internal/telemetry"
)

// buildSidecar loads config, wires components and MCP servers, and returns a
// ready-to-serve *http.Server plus a shutdown function that flushes telemetry
// and closes MCP subprocesses.
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
			Defaults: conversation.ComponentDefaults{
				Temperature:      comp.Defaults.Temperature,
				MaxTokens:        comp.Defaults.MaxTokens,
				TopP:             comp.Defaults.TopP,
				TopK:             comp.Defaults.TopK,
				Stop:             comp.Defaults.Stop,
				FrequencyPenalty: comp.Defaults.FrequencyPenalty,
				PresencePenalty:  comp.Defaults.PresencePenalty,
				Seed:             comp.Defaults.Seed,
				System:           comp.Defaults.System,
			},
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

	// Connect to configured MCP servers. Failures are non-fatal so the sidecar
	// still starts if an MCP server is unavailable.
	mcpClients := make([]*mcp.Client, 0, len(cfg.MCPServers))
	for _, mcpSrv := range cfg.MCPServers {
		client, err := mcp.NewStdioClient(context.Background(), mcpSrv.Name, mcpSrv.Command)
		if err != nil {
			slog.Warn("could not connect to MCP server", "name", mcpSrv.Name, "err", err)
			continue
		}
		mcpClients = append(mcpClients, client)
		slog.Info("connected to MCP server", "name", mcpSrv.Name)
	}

	srv = &http.Server{
		Addr:    fmt.Sprintf("127.0.0.1:%d", cfg.Port),
		Handler: server.New(components, mcpClients),
	}

	shutdown = func(ctx context.Context) error {
		for _, client := range mcpClients {
			client.Close()
		}
		defer telShutdown(context.Background()) //nolint:errcheck
		return srv.Shutdown(ctx)
	}

	return srv, shutdown, nil
}
