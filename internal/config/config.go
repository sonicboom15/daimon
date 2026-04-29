// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

// Package config loads and validates the daimon YAML configuration.
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Component is a single configured provider instance.
type Component struct {
	Name     string            `yaml:"name"`
	Type     string            `yaml:"type"`
	Metadata map[string]string `yaml:"metadata"`
}

// Telemetry holds OpenTelemetry configuration.
type Telemetry struct {
	OTLPEndpoint string `yaml:"otlp_endpoint"`
}

// Config is the top-level sidecar configuration.
type Config struct {
	Port       int         `yaml:"port"`
	Components []Component `yaml:"components"`
	Telemetry  Telemetry   `yaml:"telemetry"`
}

// Load reads and parses the YAML config file at path.
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
	return cfg, nil
}
