// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	f := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(f, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestLoad_Valid(t *testing.T) {
	path := writeConfig(t, `
port: 8080
components:
  - name: gpt4o
    type: openai
    metadata:
      default_model: gpt-4o
  - name: claude
    type: anthropic
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != 8080 {
		t.Errorf("port = %d, want 8080", cfg.Port)
	}
	if len(cfg.Components) != 2 {
		t.Errorf("components = %d, want 2", len(cfg.Components))
	}
}

func TestLoad_PortDefaultsTo3500(t *testing.T) {
	path := writeConfig(t, `
components:
  - name: local
    type: llamacpp
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != 3500 {
		t.Errorf("port = %d, want 3500", cfg.Port)
	}
}

func TestLoad_EmptyFile(t *testing.T) {
	path := writeConfig(t, "")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != 3500 {
		t.Errorf("port = %d, want 3500", cfg.Port)
	}
	if len(cfg.Components) != 0 {
		t.Errorf("components = %d, want 0", len(cfg.Components))
	}
}

func TestLoad_Defaults(t *testing.T) {
	temp := 0.7
	path := writeConfig(t, `
components:
  - name: gpt4o
    type: openai
    defaults:
      temperature: 0.7
      max_tokens: 1024
      system: "You are helpful."
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	d := cfg.Components[0].Defaults
	if d.Temperature == nil || *d.Temperature != temp {
		t.Errorf("temperature = %v, want %v", d.Temperature, temp)
	}
	if d.MaxTokens != 1024 {
		t.Errorf("max_tokens = %d, want 1024", d.MaxTokens)
	}
	if d.System != "You are helpful." {
		t.Errorf("system = %q", d.System)
	}
}

func TestLoad_MCPServers(t *testing.T) {
	path := writeConfig(t, `
components:
  - name: gpt4o
    type: openai
mcp_servers:
  - name: filesystem
    command: ["npx", "-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.MCPServers) != 1 {
		t.Fatalf("mcp_servers = %d, want 1", len(cfg.MCPServers))
	}
	if cfg.MCPServers[0].Name != "filesystem" {
		t.Errorf("mcp server name = %q", cfg.MCPServers[0].Name)
	}
}

func TestLoad_Errors(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{
			name:    "missing component name",
			content: "components:\n  - type: openai\n",
			wantErr: "missing name",
		},
		{
			name:    "missing component type",
			content: "components:\n  - name: gpt4o\n",
			wantErr: "missing type",
		},
		{
			name:    "duplicate component name",
			content: "components:\n  - name: gpt4o\n    type: openai\n  - name: gpt4o\n    type: anthropic\n",
			wantErr: "duplicate component name",
		},
		{
			name:    "bad yaml",
			content: "port: [not an int",
			wantErr: "parsing config",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := writeConfig(t, tc.content)
			_, err := Load(path)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if tc.wantErr != "" && !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error = %q, want to contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

