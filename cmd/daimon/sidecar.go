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
	"github.com/sonicboom15/daimon/internal/embedding"
	"github.com/sonicboom15/daimon/internal/mcp"
	"github.com/sonicboom15/daimon/internal/memory"
	"github.com/sonicboom15/daimon/internal/server"
	"github.com/sonicboom15/daimon/internal/session"
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

	// Wiring order: embedders → session store → vector stores → graph stores → LLM components.
	// Each layer can only reference names resolved in a prior layer.
	embedders := make(map[string]embedding.Embedder)
	var sessionSt session.SessionStore = session.NewInMemory()
	sessionConfigured := false
	vectorStores := make(map[string]memory.MemoryStore)
	graphStores := make(map[string]memory.GraphStore)
	llmComponents := make(map[string]conversation.Conversation)
	componentStores := make(map[string]string) // LLM component name → vector store name

	for _, comp := range cfg.Components {
		baseCfg := comp.Metadata
		if baseCfg == nil {
			baseCfg = map[string]string{}
		}

		// 1. Embedding registry.
		if emb, embErr := embedding.New(comp.Type, embedding.EmbedConfig{Metadata: baseCfg}); embErr == nil {
			embedders[comp.Name] = emb
			slog.Info("registered embedder", "name", comp.Name, "type", comp.Type)
			continue
		}

		// 2. Session registry (at most one session store).
		if ss, sessErr := session.New(comp.Type, session.SessionConfig{Metadata: baseCfg}); sessErr == nil {
			if sessionConfigured {
				_ = telShutdown(context.Background())
				return nil, nil, fmt.Errorf("only one session store may be configured (found %q)", comp.Name)
			}
			sessionSt = ss
			sessionConfigured = true
			slog.Info("registered session store", "name", comp.Name, "type", comp.Type)
			continue
		}

		// 3. Vector store registry — resolve embedder if named in metadata.
		var emb embedding.Embedder
		if embName := baseCfg["embedder"]; embName != "" {
			e, ok := embedders[embName]
			if !ok {
				_ = telShutdown(context.Background())
				return nil, nil, fmt.Errorf("vector store %q references unknown embedder %q (declare it before this component)", comp.Name, embName)
			}
			emb = e
		}
		storeCfg := memory.StoreConfig{Metadata: baseCfg, Embedder: emb}
		if ms, storeErr := memory.New(comp.Type, storeCfg); storeErr == nil {
			vectorStores[comp.Name] = ms
			slog.Info("registered vector store", "name", comp.Name, "type", comp.Type)
			continue
		}

		// 4. Graph store registry.
		if gs, graphErr := memory.NewGraph(comp.Type, memory.StoreConfig{Metadata: baseCfg}); graphErr == nil {
			graphStores[comp.Name] = gs
			slog.Info("registered graph store", "name", comp.Name, "type", comp.Type)
			continue
		}

		// 5. LLM registry (existing path).
		compCfg := conversation.ComponentConfig{
			Metadata:    baseCfg,
			Models:      make(map[string]conversation.ModelConfig, len(comp.Models)),
			MemoryStore: comp.MemoryStore,
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
		c, compErr := conversation.New(comp.Type, compCfg)
		if compErr != nil {
			_ = telShutdown(context.Background())
			return nil, nil, fmt.Errorf("creating component %q: %w", comp.Name, compErr)
		}
		llmComponents[comp.Name] = c
		slog.Info("registered LLM component", "name", comp.Name, "type", comp.Type)

		if comp.MemoryStore != "" {
			if _, ok := vectorStores[comp.MemoryStore]; !ok {
				_ = telShutdown(context.Background())
				return nil, nil, fmt.Errorf("component %q references unknown vector store %q", comp.Name, comp.MemoryStore)
			}
			componentStores[comp.Name] = comp.MemoryStore
		}
	}

	// Connect to configured MCP servers.
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
		Addr: fmt.Sprintf("127.0.0.1:%d", cfg.Port),
		Handler: server.New(
			llmComponents,
			mcpClients,
			vectorStores,
			graphStores,
			componentStores,
			sessionSt,
		),
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
