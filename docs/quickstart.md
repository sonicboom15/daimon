---
hide:
  - navigation
---

# Quick Start

Get daimon running and make your first streaming request in under five minutes.

## 1. Install

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
    sudo dpkg -i daimon_*_linux_amd64.deb
    ```

=== "Build from source"

    Requires Go 1.23+.

    ```bash
    git clone https://github.com/sonicboom15/daimon.git
    cd daimon && make build
    # binary at ./bin/daimon
    ```

---

## 2. Set up a model

=== "Anthropic / OpenAI"

    Export your API key, then save a `config.yaml`:

    ```bash
    export ANTHROPIC_API_KEY=sk-ant-...
    # export OPENAI_API_KEY=sk-...
    ```

    ```yaml title="config.yaml"
    port: 3500

    components:
      - name: claude
        type: anthropic
        metadata:
          default_model: claude-haiku-4-5-20251001

      - name: gpt4o
        type: openai
        metadata:
          default_model: gpt-4o-mini
    ```

=== "Local model (Docker — no API key)"

    Start [Ollama](https://ollama.com) in Docker and pull a model:

    ```bash
    docker run -d -p 11434:11434 --name ollama ollama/ollama
    docker exec ollama ollama pull qwen2.5:1.5b
    ```

    Save a `config.yaml` pointing at it:

    ```yaml title="config.yaml"
    port: 3500

    components:
      - name: local
        type: llamacpp
        metadata:
          base_url: http://localhost:11434/v1
          default_model: qwen2.5:1.5b
    ```

    !!! tip
        Swap `qwen2.5:1.5b` for any model on [ollama.com/library](https://ollama.com/library). Larger models are slower but more capable.

---

## 3. Start daimon

```bash
daimon serve --config config.yaml
```

```
INFO daimon listening addr=127.0.0.1:3500
```

---

## 4. Make a request

!!! note
    Examples below use `claude`. If you used the Docker setup, replace it with `local`.

=== "Python SDK"

    ```bash
    pip install daimon-client
    ```

    ```python
    import daimon_client as daimon

    with daimon.Client() as client:
        for text in client.llm().stream("What is a daimon?"):
            print(text, end="", flush=True)
    ```

=== "TypeScript SDK"

    ```bash
    npm install daimon-client
    ```

    ```typescript
    import { Client } from 'daimon-client';

    const client = new Client();
    for await (const text of client.llm().stream('What is a daimon?')) {
      process.stdout.write(text);
    }
    ```

=== "Python (async)"

    ```python
    import asyncio
    import daimon_client as daimon

    async def main():
        async with daimon.AsyncClient() as client:
            async for text in client.llm().stream("What is a daimon?"):
                print(text, end="", flush=True)

    asyncio.run(main())
    ```

---

## Next steps

<div class="grid cards" markdown>

-   :material-cog:{ .lg .middle } **Configuration**

    ---

    Components, inference defaults, MCP servers, telemetry — all in one YAML.

    [:octicons-arrow-right-24: Configuration](configuration/index.md)

-   :material-language-python:{ .lg .middle } **Python SDK**

    ---

    Multi-turn conversations, sessions, tool calls, async.

    [:octicons-arrow-right-24: Python SDK](sdk/python.md)

-   :material-language-typescript:{ .lg .middle } **TypeScript SDK**

    ---

    Native fetch, async generators, full type safety.

    [:octicons-arrow-right-24: TypeScript SDK](sdk/typescript.md)

-   :material-tools:{ .lg .middle } **Tool Calls (MCP)**

    ---

    Wire up filesystem, GitHub, search, and custom tools.

    [:octicons-arrow-right-24: MCP tools](mcp.md)

</div>
