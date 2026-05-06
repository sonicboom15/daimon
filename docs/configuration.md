# Configuration

Daimon is configured via a single YAML file, passed with `--config`. By default it looks for `examples/config.yaml`.

```bash
daimon serve --config /path/to/config.yaml
```

---

## Top-level structure

```yaml
port: 3500          # (int) Port to listen on. Default: 3500.

components:         # List of provider components.
  - ...

mcp_servers:        # (optional) MCP tool servers to connect to at startup.
  - ...

telemetry:          # (optional) OpenTelemetry configuration.
  otlp_endpoint: ""
```

---

## `components`

Each entry defines one named provider instance. The `name` is used in request URLs: `POST /v1/converse/{name}`.

```yaml
components:
  - name: claude          # (required) Unique name; used in the URL path.
    type: anthropic       # (required) Provider type. See Providers.
    metadata:             # Provider-specific settings.
      default_model: claude-opus-4-7
      api_key: sk-ant-... # Falls back to ANTHROPIC_API_KEY env var.
    models:               # (optional) Per-model API key overrides.
      claude-haiku-4-5-20251001:
        api_key: sk-ant-haiku-key
    defaults:             # (optional) Inference parameter defaults.
      temperature: 1.0
      max_tokens: 4096
```

### Inference parameter defaults

All fields under `defaults:` are optional. Request values always take precedence over these. Zero / nil means "use the provider's own default".

| Field | Type | Providers | Description |
|---|---|---|---|
| `temperature` | float | all | Sampling temperature |
| `max_tokens` | int | all | Maximum output tokens |
| `top_p` | float | all | Nucleus sampling probability |
| `top_k` | int | Anthropic | Top-K sampling |
| `stop` | list of strings | all | Stop sequences |
| `frequency_penalty` | float | OpenAI, llamacpp | Frequency penalty |
| `presence_penalty` | float | OpenAI, llamacpp | Presence penalty |
| `seed` | int | OpenAI, llamacpp | RNG seed for reproducibility |
| `system` | string | all | Default system prompt (used only when the request contains no system message) |

```yaml
defaults:
  temperature: 0.7
  max_tokens: 2048
  top_p: 0.9
  stop: ["Human:", "Assistant:"]
  system: "You are a helpful assistant."
```

---

## `mcp_servers`

Daimon connects to each listed MCP server at startup (as a client, over stdio). Their tools are injected into every chat request automatically.

```yaml
mcp_servers:
  - name: filesystem    # Logical name (used in logs).
    command:            # Command to start the server process.
      - npx
      - -y
      - "@modelcontextprotocol/server-filesystem"
      - /tmp

  - name: github
    command:
      - npx
      - -y
      - "@modelcontextprotocol/server-github"
```

!!! note
    MCP server startup failures are non-fatal. Daimon logs a warning and continues without that server's tools.

See [Tool Calls (MCP)](mcp.md) for the full guide.

---

## `telemetry`

```yaml
telemetry:
  otlp_endpoint: "localhost:4318"   # OTLP HTTP endpoint. Leave empty to disable.
```

When `otlp_endpoint` is set, daimon sends OpenTelemetry traces via OTLP/HTTP to the given address. Each HTTP request to `/v1/converse/{component}` creates a root span.

See [Observability](observability.md) for details.

---

## Full example

```yaml
port: 3500

components:
  - name: claude
    type: anthropic
    metadata:
      default_model: claude-opus-4-7
      # api_key: sk-ant-...
    defaults:
      temperature: 1.0
      max_tokens: 4096
      system: "You are a helpful assistant."

  - name: gpt4o
    type: openai
    metadata:
      default_model: gpt-4o
      # api_key: sk-...
    defaults:
      temperature: 0.7
      max_tokens: 2048
      seed: 42

  - name: local
    type: llamacpp
    metadata:
      base_url: http://localhost:11434/v1   # Ollama
      default_model: llama3.2:3b

mcp_servers:
  - name: filesystem
    command: ["npx", "-y", "@modelcontextprotocol/server-filesystem", "/tmp"]

telemetry:
  otlp_endpoint: "localhost:4318"
```
