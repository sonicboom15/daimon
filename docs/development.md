---
hide:
  - navigation
---

# Development

## Prerequisites

- Go 1.23+
- Make
- (Optional) Docker — for llamacpp integration tests

## Build from source

```bash
git clone https://github.com/sonicboom15/daimon.git
cd daimon
make build          # → ./bin/daimon
```

## Make targets

| Target | Description |
|---|---|
| `make build` | Compile to `./bin/daimon` |
| `make run` | Build and run with `examples/config.yaml` |
| `make test` | Run all unit tests |
| `make lint` | Run `golangci-lint` |
| `make fmt` | Run `gofmt` + `goimports` |
| `make license-check` | Verify Apache 2.0 headers on all source files |

## Testing

### Unit tests (no API keys needed)

```bash
go test ./...
go test -race ./internal/...   # with race detector
```

Unit tests use in-process fakes — no real API calls, no Docker.

### Integration tests

Integration tests make real API calls or start Docker containers. They are gated by the `integration` build tag.

```bash
# OpenAI + Anthropic
OPENAI_API_KEY=sk-... ANTHROPIC_API_KEY=sk-ant-... \
  go test -tags integration -v ./internal/components/...

# llamacpp only — starts Ollama in Docker, pulls qwen2.5:1.5b
go test -tags integration -v ./internal/components/llamacpp/

# Use a different model (e.g. one that supports tool calls)
DAIMON_OLLAMA_MODEL=llama3.2:1b \
  go test -tags integration -v ./internal/components/llamacpp/
```

The llamacpp test skips gracefully if Docker is not available.

### Python SDK tests

```bash
cd sdk/python
pip install -e ".[dev]"
pytest tests/ -v
```

## Project structure

```
cmd/daimon/              # CLI entry point (serve, run)
internal/
  config/                # YAML config loader + validation
  server/                # HTTP routing, SSE handler, agentic loop
  conversation/          # Conversation interface, types, registry
  components/
    openai/              # OpenAI provider
    anthropic/           # Anthropic provider
    llamacpp/            # OpenAI-compatible local server provider
  mcp/                   # MCP stdio client (JSON-RPC 2.0)
  telemetry/             # OpenTelemetry setup
sdk/python/              # daimon-client Python package
  daimon_client/
  tests/
examples/
  config.yaml            # Sample config with all options documented
  client/                # Runnable example scripts
docs/                    # This documentation site (MkDocs)
.github/workflows/       # CI: release, docs deploy, Python publish
```

## Adding a provider

1. Create `internal/components/<name>/<name>.go`.
2. Implement the `conversation.Conversation` interface:
   ```go
   func (c *Component) Chat(ctx context.Context, req conversation.Request) (<-chan conversation.Chunk, error) {
       ch := make(chan conversation.Chunk)
       go func() {
           defer close(ch)
           // call upstream API, stream chunks...
           ch <- conversation.Chunk{Type: conversation.ChunkDone}
       }()
       return ch, nil
   }
   ```
3. Register in `init()`:
   ```go
   func init() {
       conversation.Register("<name>", func(cfg conversation.ComponentConfig) (conversation.Conversation, error) {
           return New(cfg)
       })
   }
   ```
4. Add blank imports to `cmd/daimon/serve.go` and `cmd/daimon/run.go`:
   ```go
   _ "github.com/sonicboom15/daimon/internal/components/<name>"
   ```
5. Add an entry to `examples/config.yaml`.
6. Write an integration test in `<name>_integration_test.go` (tagged `//go:build integration`).

!!! important "Architecture rules"
    - Components must not import `internal/server` or `internal/config` — only `internal/conversation`.
    - Helper functions (`first`, `firstSlice`, `effectiveSystem`) are intentionally duplicated per-component to keep them self-contained. Do not move them to a shared package.

## Docs site

The documentation site (this site) is built with [MkDocs Material](https://squidfunk.github.io/mkdocs-material/).

```bash
pip install mkdocs-material
mkdocs serve      # live preview at http://localhost:8000
mkdocs build      # build static site to ./site/
```

It deploys automatically to GitHub Pages when `docs/**` or `mkdocs.yml` changes on `main`.

## License

Apache 2.0. All source files must begin with:

```go
// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0
```

Run `make license-check` to verify.
