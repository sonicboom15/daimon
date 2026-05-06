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

## 2. Create a config file

Save this as `config.yaml`. You only need the provider(s) you have keys for.

```yaml
port: 3500

components:
  - name: claude
    type: anthropic
    metadata:
      default_model: claude-haiku-4-5-20251001
      # api_key: sk-ant-...  # or set ANTHROPIC_API_KEY

  - name: gpt4o
    type: openai
    metadata:
      default_model: gpt-4o-mini
      # api_key: sk-...  # or set OPENAI_API_KEY
```

---

## 3. Start daimon

```bash
export ANTHROPIC_API_KEY=sk-ant-...
export OPENAI_API_KEY=sk-...

daimon serve --config config.yaml
```

You should see:

```
INFO daimon listening addr=127.0.0.1:3500
```

---

## 4. Make a request

=== "curl"

    ```bash
    curl -sN http://127.0.0.1:3500/v1/converse/claude \
      -H "Content-Type: application/json" \
      -d '{"messages":[{"role":"user","content":"What is a daimon?"}]}'
    ```

    ```
    data: {"type":"text","text":"In ancient Greek thought, a daimon"}
    data: {"type":"text","text":" is a guiding spirit or divine force..."}
    data: {"type":"done"}
    ```

=== "Python SDK"

    ```bash
    pip install daimon-client
    ```

    ```python
    import daimon_client as daimon

    with daimon.Client() as client:
        for text in client.stream("claude", "What is a daimon?"):
            print(text, end="", flush=True)
    ```

=== "Python (async)"

    ```python
    import asyncio
    import daimon_client as daimon

    async def main():
        async with daimon.AsyncClient() as client:
            async for text in client.stream("claude", "What is a daimon?"):
                print(text, end="", flush=True)

    asyncio.run(main())
    ```

=== "Any HTTP client"

    Daimon speaks plain HTTP + SSE. Use any language:

    ```js
    // Node.js (using fetch)
    const res = await fetch("http://127.0.0.1:3500/v1/converse/claude", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ messages: [{ role: "user", content: "Hello" }] }),
    });
    for await (const chunk of res.body) {
      const line = new TextDecoder().decode(chunk);
      if (line.startsWith("data: ")) {
        const { type, text } = JSON.parse(line.slice(6));
        if (type === "text") process.stdout.write(text);
      }
    }
    ```

---

## Next steps

- [Full configuration reference](configuration.md) — defaults, MCP servers, telemetry
- [Python SDK](sdk/python.md) — multi-turn conversations, tool calls, async
- [Tool calls via MCP](mcp.md) — wire up filesystem, GitHub, or custom tools
