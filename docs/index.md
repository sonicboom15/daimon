---
hide:
  - navigation
  - toc
---

<div align="center" markdown>

# Daimon

**The spirit that runs alongside your AI app.**

Daimon is a local sidecar process that gives your application a **single, stable HTTP interface** to any LLM. Swap providers, rotate keys, add tracing, or wire up MCP tool servers — without touching your application code.

```
your app  ──POST /v1/converse/claude──▶  daimon  ──▶  Anthropic API
          ◀── text/event-stream ────────────────────────────────────
                                              ▼
                                       MCP tool server(s)
```

</div>

---

## Why Daimon?

| Without daimon | With daimon |
|---|---|
| Provider SDKs in every service | One HTTP call from anywhere |
| API keys scattered across code | Keys live in one config file |
| Changing providers = code changes | Change a line in YAML |
| No tracing without instrumentation | OpenTelemetry built in |
| Tool calls require orchestration code | MCP agentic loop handled for you |

---

## Key features

- **Streaming-first** — responses arrive as Server-Sent Events, token by token
- **Provider-agnostic** — OpenAI, Anthropic, and any OpenAI-compatible server (Ollama, LM Studio, llama.cpp)
- **Inference parameter defaults** — set temperature, max_tokens, system prompt, and more per-component in YAML; override per-request at runtime
- **Server-side sessions** — pass a `session_id` and daimon maintains conversation history for you; clients only send the new turn
- **MCP tool calls** — configure MCP servers in YAML; daimon injects their tools into every request and drives the full agentic loop transparently
- **Python SDK** — `pip install daimon-client` for sync and async streaming clients
- **TypeScript SDK** — `npm install daimon-client` for Node.js and edge runtimes
- **OpenTelemetry tracing** — structured traces per request, compatible with any OTLP collector

---

## Install

=== "macOS / Linux"

    ```bash
    brew tap sonicboom15/tap
    brew install daimon
    ```

=== "Windows (winget)"

    ```powershell
    winget install sonicboom15.daimon
    ```

=== "Windows (Scoop)"

    ```powershell
    scoop bucket add sonicboom15 https://github.com/sonicboom15/scoop-bucket
    scoop install daimon
    ```

=== "Linux (.deb / .rpm)"

    Download from the [latest release](https://github.com/sonicboom15/daimon/releases/latest).

    ```bash
    sudo dpkg -i daimon_*_linux_amd64.deb   # Debian / Ubuntu
    sudo rpm -i  daimon_*_linux_amd64.rpm   # RHEL / Fedora
    ```

---

## First request in 60 seconds

<div align="center" markdown>

[Get started →](quickstart.md){ .md-button .md-button--primary }
[API reference →](api.md){ .md-button }

</div>

---

<div align="center" markdown>

## Explore the docs

</div>

<div class="grid cards" markdown>

-   :material-rocket-launch:{ .lg .middle } **Quick Start**

    ---

    Up and running in five minutes with a config file and your first request.

    [:octicons-arrow-right-24: Quick Start](quickstart.md)

-   :material-api:{ .lg .middle } **HTTP API**

    ---

    Full reference for `/v1/converse`, sessions, SSE chunk types, and healthz.

    [:octicons-arrow-right-24: API reference](api.md)

-   :material-language-python:{ .lg .middle } **Python SDK**

    ---

    `pip install daimon-client` — sync and async streaming clients with sessions.

    [:octicons-arrow-right-24: Python SDK](sdk/python.md)

-   :material-language-typescript:{ .lg .middle } **TypeScript SDK**

    ---

    `npm install daimon-client` — native fetch, async generators, full type safety.

    [:octicons-arrow-right-24: TypeScript SDK](sdk/typescript.md)

-   :material-wrench:{ .lg .middle } **Configuration**

    ---

    Components, inference defaults, MCP servers, and telemetry — all in one YAML file.

    [:octicons-arrow-right-24: Configuration](configuration/index.md)

-   :material-tools:{ .lg .middle } **Tool Calls (MCP)**

    ---

    Wire up filesystem, GitHub, search, and custom MCP servers with zero client code.

    [:octicons-arrow-right-24: MCP tools](mcp.md)

</div>
