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

components:         # one or more provider instances
  - ...

mcp_servers:        # (optional) MCP tool servers
  - ...

telemetry:          # (optional) OpenTelemetry
  otlp_endpoint: ""
```

---

## Full example

```yaml
port: 3500

components:
  - name: claude
    type: anthropic
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

    Provider instances, per-model API keys, and inference parameter defaults.

    [:octicons-arrow-right-24: Components](components.md)

-   :material-tools:{ .lg .middle } **MCP Servers**

    ---

    Connect tool servers at startup and let the model call them automatically.

    [:octicons-arrow-right-24: MCP Servers](mcp-servers.md)

-   :material-chart-line:{ .lg .middle } **Telemetry**

    ---

    OpenTelemetry tracing via OTLP — zero overhead when not configured.

    [:octicons-arrow-right-24: Telemetry](telemetry.md)

</div>
