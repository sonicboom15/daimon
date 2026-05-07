---
hide:
  - toc
---

# Configuration

Daimon is configured via a single YAML file passed with `--config`. By default it looks for `examples/config.yaml`.

```bash
daimon serve --config /path/to/config.yaml
```

---

## Structure

```yaml
port: 3500          # port to listen on (default: 3500)

components:         # LLMs, embedders, session stores, vector stores, and graph stores
  - ...

mcp_servers:        # (optional) MCP tool servers
  - ...

telemetry:          # (optional) OpenTelemetry
  otlp_endpoint: ""
```

All component types live under `components:`. The sidecar auto-detects the type by trying each registry in order: **embedding → session → vector → graph → LLM**. Declaration order matters: embedders must appear before vector stores that use them, and vector stores before LLM components that reference them via `memory_store:`.

---

## Full example

```yaml
port: 3500

components:

  # ── Embedder (declare before vector stores that reference it) ───────────────
  - name: embedder
    type: embedding/openai
    metadata:
      base_url: http://localhost:11434/v1   # Ollama — omit for OpenAI
      model: nomic-embed-text
      dimensions: "768"

  # ── Session store (optional; defaults to in-memory) ─────────────────────────
  - name: sessions
    type: session/redis
    metadata:
      addr: localhost:6379
      ttl: "24h"

  # ── Vector / document stores ────────────────────────────────────────────────
  - name: docs
    type: inmemory        # BM25 lexical — no external service

  - name: chroma-docs
    type: chroma
    metadata:
      base_url: http://localhost:8000
      collection: daimon
      create_if_missing: "true"

  - name: qdrant-docs
    type: qdrant
    metadata:
      base_url: http://localhost:6333
      collection: daimon
      embedder: embedder           # reference by component name
      create_if_missing: "true"

  # ── Graph stores ────────────────────────────────────────────────────────────
  - name: kg
    type: neo4j
    metadata:
      bolt_url: bolt://localhost:7687
      database: neo4j
      username: neo4j
      password: secret

  # ── LLM components ──────────────────────────────────────────────────────────
  - name: claude
    type: anthropic
    memory_store: chroma-docs    # transparent RAG from chroma-docs on every request
    metadata:
      default_model: claude-opus-4-7
      # api_key: sk-ant-...  # or ANTHROPIC_API_KEY
    defaults:
      temperature: 1.0
      max_tokens: 4096
      system: "You are a helpful assistant."

  - name: gpt4o
    type: openai
    metadata:
      default_model: gpt-4o
      # api_key: sk-...  # or OPENAI_API_KEY
    defaults:
      temperature: 0.7
      max_tokens: 2048
      seed: 42

  - name: local
    type: llamacpp
    metadata:
      base_url: http://localhost:11434/v1   # Ollama
      default_model: qwen2.5:1.5b

mcp_servers:
  - name: filesystem
    command: ["npx", "-y", "@modelcontextprotocol/server-filesystem", "/tmp"]

telemetry:
  otlp_endpoint: "localhost:4318"
```

---

<div class="grid cards" markdown>

-   :material-server:{ .lg .middle } **Components**

    ---

    LLM providers, embedders, session stores, vector stores, and graph stores.

    [:octicons-arrow-right-24: Components](components.md)

-   :material-database:{ .lg .middle } **Stores**

    ---

    Vector search and graph database backends for memory and knowledge.

    [:octicons-arrow-right-24: Stores](../stores/index.md)

-   :material-tools:{ .lg .middle } **MCP Servers**

    ---

    Connect tool servers at startup and let the model call them automatically.

    [:octicons-arrow-right-24: MCP Servers](mcp-servers.md)

-   :material-chart-line:{ .lg .middle } **Telemetry**

    ---

    OpenTelemetry tracing via OTLP — zero overhead when not configured.

    [:octicons-arrow-right-24: Telemetry](telemetry.md)

</div>
