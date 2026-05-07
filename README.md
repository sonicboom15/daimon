# daimon

> The spirit that runs alongside your AI app.

[![GitHub release](https://img.shields.io/github/v/release/sonicboom15/daimon)](https://github.com/sonicboom15/daimon/releases/latest)
[![PyPI](https://img.shields.io/pypi/v/daimon-client?label=PyPI)](https://pypi.org/project/daimon-client/)
[![npm](https://img.shields.io/npm/v/daimon-client?label=npm)](https://www.npmjs.com/package/daimon-client)
[![License](https://img.shields.io/github/license/sonicboom15/daimon)](LICENSE)
[![Docs](https://img.shields.io/badge/docs-sonicboom15.github.io%2Fdaimon-blue)](https://sonicboom15.github.io/daimon/)

Daimon is a local sidecar process that gives your application a single, stable HTTP interface to any LLM. Swap providers, rotate keys, add tracing, or wire up MCP tools — without touching your app code.

Inspired by [Dapr's](https://dapr.io) component model, adapted for AI-native primitives: streaming responses, pluggable providers, MCP tool calls, and (eventually) memory and agent orchestration.

---

## How it works

```
your app  ──POST /v1/converse/claude──▶  daimon  ──▶  Anthropic API
          ◀── text/event-stream ────────────────────────────────────
                                            ▼
                                     MCP tool server(s)
                                   (filesystem, GitHub, ...)
```

Daimon runs on `localhost:3500`. Your app speaks plain HTTP + Server-Sent Events. The provider, model, credentials, and tool servers all live in a YAML config — not in your code.

---

## Quick start

**Prerequisites:** An OpenAI or Anthropic API key.

### 1 — Install

**macOS / Linux — Homebrew**
```bash
brew tap sonicboom15/tap
brew install daimon
```

**Windows — winget**
```powershell
winget install sonicboom15.daimon
```

**Windows — Scoop**
```powershell
scoop bucket add sonicboom15 https://github.com/sonicboom15/scoop-bucket
scoop install daimon
```

**Linux — apt / rpm**
Download the `.deb` or `.rpm` from the [latest release](https://github.com/sonicboom15/daimon/releases/latest) and install with `dpkg -i` or `rpm -i`.

**Build from source**
```bash
git clone https://github.com/sonicboom15/daimon.git && cd daimon && make build
# → ./bin/daimon
```

### 2 — Create a config

```yaml
# config.yaml
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

### 3 — Run

```bash
export ANTHROPIC_API_KEY=sk-ant-...
export OPENAI_API_KEY=sk-...
daimon serve --config config.yaml
```

```
INFO daimon listening addr=127.0.0.1:3500
```

### 4 — First request

**curl:**

```bash
curl -sN http://127.0.0.1:3500/v1/converse/claude \
  -H "Content-Type: application/json" \
  -d '{"messages":[{"role":"user","content":"What is a daimon?"}]}'
```

```
data: {"type":"text","text":"In ancient Greek thought, a daimon"}
data: {"type":"text","text":" is a guiding spirit..."}
data: {"type":"done"}
```

**Python SDK:**

```bash
pip install daimon-client
```

```python
import daimon_client as daimon

with daimon.Client() as client:
    for text in client.stream("claude", "What is a daimon?"):
        print(text, end="", flush=True)
```

**TypeScript SDK:**

```bash
npm install daimon-client
```

```typescript
import { Client } from 'daimon-client';

const client = new Client();
for await (const text of client.stream('claude', 'What is a daimon?')) {
  process.stdout.write(text);
}
```

---

## Configuration

```yaml
port: 3500

components:
  - name: claude
    type: anthropic
    metadata:
      default_model: claude-opus-4-7
      # api_key: sk-ant-...  # or set ANTHROPIC_API_KEY
    # Per-request defaults (all optional — request values always win):
    # defaults:
    #   temperature: 1.0
    #   max_tokens: 4096
    #   top_p: 0.9
    #   top_k: 50          # Anthropic-specific
    #   stop: ["Human:"]
    #   system: "You are a helpful assistant."

  - name: gpt4o
    type: openai
    metadata:
      default_model: gpt-4o
      # api_key: sk-...  # or set OPENAI_API_KEY
    # defaults:
    #   temperature: 0.7
    #   max_tokens: 2048
    #   frequency_penalty: 0.0
    #   presence_penalty: 0.0
    #   seed: 42

  - name: local
    type: llamacpp
    metadata:
      base_url: http://localhost:11434/v1   # Ollama default
      # base_url: http://localhost:1234/v1  # LM Studio default
      # base_url: http://localhost:8080/v1  # llama.cpp default
      default_model: llama3.2:3b

# MCP tool servers — daimon connects at startup and injects their tools into
# every chat request automatically. The model can call them; daimon runs the loop.
# mcp_servers:
#   - name: filesystem
#     command: ["npx", "-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
#   - name: github
#     command: ["npx", "-y", "@modelcontextprotocol/server-github"]

telemetry:
  otlp_endpoint: ""   # e.g. "localhost:4318" — leave empty to disable
```

Each component has a **name** (used in the request URL) and a **type** (selects the provider). Inference parameters set under `defaults:` are used when the request doesn't supply them — request values always win.

---

## API

### `POST /v1/converse/{component}`

Send a chat request and receive a streaming response over Server-Sent Events.

**Request body:**

```json
{
  "messages": [
    { "role": "system",    "content": "You are a helpful assistant." },
    { "role": "user",      "content": "What is a daimon?" }
  ],
  "model":             "gpt-4o-mini",
  "system":            "Override or set a system prompt here.",
  "max_tokens":        512,
  "temperature":       0.7,
  "top_p":             0.9,
  "top_k":             50,
  "stop":              ["Human:"],
  "frequency_penalty": 0.0,
  "presence_penalty":  0.0,
  "seed":              42,
  "tools": [
    {
      "name":        "get_weather",
      "description": "Get current weather for a city.",
      "input_schema": {
        "type": "object",
        "properties": { "city": { "type": "string" } },
        "required":   ["city"]
      }
    }
  ]
}
```

All fields except `messages` are optional. Omitted inference parameters fall back to the component's configured defaults.

**Sessions:** include `"session_id"` to have daimon maintain conversation history server-side. Only send the new user turn — the server prepends stored history automatically.

```bash
# Turn 1
curl -sN http://127.0.0.1:3500/v1/converse/claude \
  -H "Content-Type: application/json" \
  -d '{"session_id":"chat-1","messages":[{"role":"user","content":"My name is Alice."}]}'

# Turn 2 — server prepends the previous exchange automatically
curl -sN http://127.0.0.1:3500/v1/converse/claude \
  -H "Content-Type: application/json" \
  -d '{"session_id":"chat-1","messages":[{"role":"user","content":"What is my name?"}]}'
```

Clear a session with `DELETE /v1/sessions/{id}` (returns `204`, idempotent).

**Provider support matrix:**

| Parameter | OpenAI | Anthropic | llamacpp |
|---|---|---|---|
| `temperature` | ✓ | ✓ | ✓ |
| `max_tokens` | ✓ | ✓ | ✓ |
| `top_p` | ✓ | ✓ | ✓ |
| `top_k` | — | ✓ | — |
| `stop` | ✓ | ✓ | ✓ |
| `frequency_penalty` | ✓ | — | ✓ |
| `presence_penalty` | ✓ | — | ✓ |
| `seed` | ✓ | — | ✓ |

Unsupported parameters are silently ignored per provider.

**Response** (`text/event-stream`):

```
data: {"type":"text","text":"In ancient Greek thought..."}

data: {"type":"tool_call","tool_call":{"id":"call_1","name":"get_weather","input":{"city":"London"}}}

data: {"type":"text","text":"The weather in London is 12°C."}

data: {"type":"done"}
```

Each `data:` line is a JSON object:

| `type` | additional fields | meaning |
|--------|-------------------|---------|
| `text` | `text` | a fragment of the model's response |
| `tool_call` | `tool_call.id`, `.name`, `.input` | model invoked a tool (daimon executes it and continues) |
| `done` | — | stream finished successfully |
| `error` | `error` | terminal error; stream ends |

`tool_call` events are forwarded so clients can show progress ("calling tool X…"). Daimon executes the tool automatically and loops back to the model — no client-side action needed.

### `DELETE /v1/sessions/{id}`

Clears server-side session history for the given ID. Returns `204 No Content`. Idempotent — deleting a session that does not exist is not an error.

### `GET /healthz`

Returns `200 ok` when the sidecar is up.

---

## Python SDK

Install:

```bash
pip install daimon-client
```

**Streaming text:**

```python
import daimon_client as daimon

# context manager reuses the HTTP connection
with daimon.Client() as client:
    for text in client.stream("claude", "Explain recursion in one sentence."):
        print(text, end="", flush=True)
print()
```

**Convenience: collect the full response:**

```python
reply = client.chat("gpt4o", "What is the capital of France?")
print(reply)  # "The capital of France is Paris."
```

**Multi-turn conversation:**

```python
messages = [
    daimon.Message(role="system", content="You are a helpful assistant."),
    daimon.Message(role="user",   content="My name is Alice."),
]
reply = client.chat("claude", messages)
messages.append(daimon.Message(role="assistant", content=reply))
messages.append(daimon.Message(role="user", content="What is my name?"))
print(client.chat("claude", messages))
```

**Sessions:**

```python
client.chat("claude", "My name is Alice.", session_id="chat-1")
reply = client.chat("claude", "What is my name?", session_id="chat-1")
# reply: "Your name is Alice."
client.clear_session("chat-1")
```

**With inference parameters:**

```python
reply = client.chat(
    "gpt4o",
    "Write a haiku about Go.",
    model="gpt-4o",
    temperature=0.9,
    max_tokens=64,
)
```

**Observing tool calls:**

```python
def on_tool(tc: daimon.ToolCall) -> None:
    print(f"[tool: {tc.name}({tc.input})]")

for text in client.stream("claude", "What's the weather in Tokyo?", on_tool_call=on_tool):
    print(text, end="", flush=True)
```

**Async:**

```python
import asyncio
import daimon_client as daimon

async def main():
    async with daimon.AsyncClient() as client:
        async for text in client.stream("claude", "Hello!"):
            print(text, end="", flush=True)

asyncio.run(main())
```

Full runnable examples: [`examples/client/chat.py`](examples/client/chat.py) · [`examples/client/chat_async.py`](examples/client/chat_async.py)

---

## TypeScript SDK

Install:

```bash
npm install daimon-client
```

**Streaming text:**

```typescript
import { Client } from 'daimon-client';

const client = new Client();
for await (const text of client.stream('claude', 'Explain recursion in one sentence.')) {
  process.stdout.write(text);
}
```

**Convenience: collect the full response:**

```typescript
const reply = await client.chat('gpt4o', 'What is the capital of France?');
console.log(reply); // "The capital of France is Paris."
```

**Sessions:**

```typescript
await client.chat('claude', 'My name is Alice.', { session_id: 'chat-1' });
const reply = await client.chat('claude', 'What is my name?', { session_id: 'chat-1' });
// reply: "Your name is Alice."
await client.clearSession('chat-1');
```

**With inference parameters:**

```typescript
const reply = await client.chat('gpt4o', 'Write a haiku about Go.', {
  model:       'gpt-4o',
  temperature: 0.9,
  max_tokens:  64,
});
```

Full runnable examples: [`sdk/typescript/examples/`](sdk/typescript/examples/)

---

## Tool calls via MCP

Daimon acts as an MCP client. Configure MCP servers in YAML and daimon:

1. Connects to each server at startup and fetches its tool catalogue.
2. Injects all tools into every chat request automatically.
3. When the model calls a tool, daimon executes it via the MCP server and feeds the result back — looping until the model returns a plain text response.

Your application sees a single streaming response with the final answer, plus `tool_call` events for progress:

```yaml
mcp_servers:
  - name: filesystem
    command: ["npx", "-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
  - name: brave-search
    command: ["npx", "-y", "@modelcontextprotocol/server-brave-search"]
```

No client-side changes required.

---

## Supported providers

| Type | Env var | Default model |
|------|---------|---------------|
| `openai` | `OPENAI_API_KEY` | `gpt-4o` |
| `anthropic` | `ANTHROPIC_API_KEY` | `claude-opus-4-7` |
| `llamacpp` | — | (required) |

`llamacpp` connects to any OpenAI-compatible local server: [llama.cpp](https://github.com/ggerganov/llama.cpp), [Ollama](https://ollama.com), or [LM Studio](https://lmstudio.ai). Set `base_url` in metadata to point at your server's `/v1` endpoint.

---

## Adding a provider

1. Create `internal/components/<name>/<name>.go`.
2. Implement `conversation.Conversation`:
   ```go
   type Component struct { /* ... */ }
   
   func (c *Component) Chat(ctx context.Context, req conversation.Request) (<-chan conversation.Chunk, error) {
       // stream chunks through the returned channel
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
4. Blank-import the package from `cmd/daimon/serve.go` and `cmd/daimon/run.go`.
5. Add a worked example to `examples/config.yaml`.

No changes to the server, config loader, or any other package.

---

## Development

```bash
make build          # compile → ./bin/daimon
make run            # build + run with examples/config.yaml
make test           # go test ./...
make lint           # golangci-lint
make fmt            # gofmt + goimports
make license-check
```

**Integration tests** (require API keys / Docker):

```bash
# OpenAI + Anthropic
OPENAI_API_KEY=sk-... ANTHROPIC_API_KEY=sk-ant-... \
  go test -tags integration -v ./internal/components/...

# llamacpp — starts Ollama in Docker automatically, pulls qwen2.5:1.5b
go test -tags integration -v ./internal/components/llamacpp/

# Full e2e suite (Go + Python SDK + TypeScript SDK) — requires Docker
go test -tags integration -v -timeout 20m ./test/e2e/
```

**Python SDK tests:**

```bash
cd sdk/python
pip install -e ".[dev]"
pytest tests/ -v
```

**TypeScript SDK tests:**

```bash
cd sdk/typescript
npm install
npm test
```

---

## Roadmap

- **Memory / vector stores** as first-class components
- **Agent orchestration** primitives (multi-step, branching)
- **Metrics** alongside traces (OTel)
- **Authentication** and rate limiting

Explicitly out of scope for now: gRPC, external plugin loading, pub/sub.

---

## License

Apache 2.0 — see [LICENSE](LICENSE).
