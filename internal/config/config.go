// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

// Package config loads and validates the daimon YAML configuration.
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ModelConfig holds per-model overrides for a component.
type ModelConfig struct {
	APIKey string `yaml:"api_key"`
}

// Component is a single configured provider instance.
// For LLM components, MemoryStore optionally names a vector store component
// that will be used for transparent RAG enrichment on every Chat call.
type Component struct {
	Name        string                 `yaml:"name"`
	Type        string                 `yaml:"type"`
	Metadata    map[string]string      `yaml:"metadata"`
	Models      map[string]ModelConfig `yaml:"models"`
	Defaults    ComponentDefaults      `yaml:"defaults"`
	MemoryStore string                 `yaml:"memory_store"`
}

// Telemetry holds OpenTelemetry configuration.
type Telemetry struct {
	OTLPEndpoint string `yaml:"otlp_endpoint"`
}

// ComponentDefaults are inference parameter defaults applied when the request
// doesn't supply that field. Request values always win.
type ComponentDefaults struct {
	Temperature      *float64 `yaml:"temperature"`
	MaxTokens        int      `yaml:"max_tokens"`
	TopP             *float64 `yaml:"top_p"`
	TopK             *int64   `yaml:"top_k"`
	Stop             []string `yaml:"stop"`
	FrequencyPenalty *float64 `yaml:"frequency_penalty"`
	PresencePenalty  *float64 `yaml:"presence_penalty"`
	Seed             *int64   `yaml:"seed"`
	System           string   `yaml:"system"`
}

// MCPServer describes an MCP server that daimon connects to as a client at startup.
type MCPServer struct {
	Name    string   `yaml:"name"`
	Command []string `yaml:"command"`
}

// Config is the top-level sidecar configuration.
type Config struct {
	Port       int         `yaml:"port"`
	Components []Component `yaml:"components"`
	MCPServers []MCPServer `yaml:"mcp_servers"`
	Telemetry  Telemetry   `yaml:"telemetry"`
}

// Load reads, parses, and validates the YAML config file at path.
// Port defaults to 3500 if not set.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}
	cfg := &Config{Port: 3500}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	if cfg.Port == 0 {
		cfg.Port = 3500
	}
	if err := validate(cfg); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	return cfg, nil
}

func validate(cfg *Config) error {
	if cfg.Port < 1 || cfg.Port > 65535 {
		return fmt.Errorf("port %d is out of range [1, 65535]", cfg.Port)
	}
	seen := make(map[string]bool, len(cfg.Components))
	for _, c := range cfg.Components {
		if c.Name == "" {
			return fmt.Errorf("component missing name")
		}
		if c.Type == "" {
			return fmt.Errorf("component %q missing type", c.Name)
		}
		if seen[c.Name] {
			return fmt.Errorf("duplicate component name %q", c.Name)
		}
		seen[c.Name] = true
	}
	return nil
}
