# daimon

> The spirit that runs alongside your AI app.

Daimon is a local sidecar process that gives your application a single, stable HTTP interface to any LLM. Swap providers, rotate keys, or add tracing — without touching your app code.

Inspired by [Dapr's](https://dapr.io) component model, adapted for AI-native primitives: streaming responses, pluggable providers, and (soon) tool calls and memory.

---

## How it works

```
your app  ──POST /v1/converse/claude──▶  daimon  ──▶  Anthropic API
          ◀── text/event-stream ─────────────────────────────────────
```

Daimon runs on `localhost:3500`. Your app speaks plain HTTP + Server-Sent Events. The provider, model, and credentials live in a YAML config file — not in your code.

---

## Quick start

**Prerequisites:** Go 1.23+, an OpenAI or Anthropic API key.

```bash
git clone https://github.com/sonicboom15/daimon.git
cd daimon
make build
```

Edit `examples/config.yaml` to add your component(s), then:

```bash
export OPENAI_API_KEY=sk-...
export ANTHROPIC_API_KEY=sk-ant-...
make run
```

The sidecar is ready when you see:

```
INFO daimon listening addr=127.0.0.1:3500
```

---

## Configuration

```yaml
port: 3500

components:
  - name: gpt4o          # the name used in request URLs
    type: openai
    metadata:
      model: gpt-4o
      # api_key: sk-...  # or set OPENAI_API_KEY

  - name: claude
    type: anthropic
    metadata:
      model: claude-opus-4-7
      # api_key: sk-ant-...  # or set ANTHROPIC_API_KEY

telemetry:
  otlp_endpoint: ""      # e.g. "localhost:4318" — leave empty to disable
```

Each entry under `components` maps a name to a provider type. The name is what you use in the request URL; the type selects which component implementation handles it.

---

## API

### `POST /v1/converse/{component}`

Send a chat request and receive a streaming response over Server-Sent Events.

**Request body:**

```json
{
  "model": "gpt-4o",
  "messages": [
    { "role": "system",    "content": "You are a helpful assistant." },
    { "role": "user",      "content": "What is a daimon?" }
  ],
  "max_tokens": 512,
  "temperature": 0.7
}
```

`model` is optional — if omitted, the component's configured default is used.

**Response** (`text/event-stream`):

```
data: {"type":"text","text":"In ancient Greek thought, a daimon"}

data: {"type":"text","text":" is a spirit or divine force..."}

data: {"type":"done"}
```

Each `data:` line is a JSON object with one of three types:

| type | fields | meaning |
|------|--------|---------|
| `text` | `text` | a fragment of the model's response |
| `done` | — | stream finished successfully |
| `error` | `error` | something went wrong; stream ends |

### `GET /healthz`

Returns `200 ok` when the sidecar is up.

---

## Calling from Python

```python
import json, requests

def chat(component, messages):
    with requests.post(
        f"http://127.0.0.1:3500/v1/converse/{component}",
        json={"messages": messages},
        stream=True,
        timeout=120,
    ) as r:
        r.raise_for_status()
        for line in r.iter_lines():
            line = line.decode() if isinstance(line, bytes) else line
            if not line.startswith("data: "):
                continue
            chunk = json.loads(line[6:])
            if chunk["type"] == "text":
                print(chunk["text"], end="", flush=True)
            elif chunk["type"] in ("done", "error"):
                break
    print()

chat("claude", [{"role": "user", "content": "Hello"}])
```

A runnable version is in [`examples/client/chat.py`](examples/client/chat.py).

---

## Supported providers

| Type | Env var | Default model |
|------|---------|---------------|
| `openai` | `OPENAI_API_KEY` | `gpt-4o` |
| `anthropic` | `ANTHROPIC_API_KEY` | `claude-opus-4-7` |

---

## Adding a provider

1. Create `internal/components/<name>/<name>.go`.
2. Implement `conversation.Conversation` — one method: `Chat(ctx, req) (<-chan Chunk, error)`.
3. Register in `init()`:
   ```go
   func init() {
       conversation.Register("myprovider", func(meta map[string]string) (conversation.Conversation, error) {
           return New(meta)
       })
   }
   ```
4. Blank-import the package from `cmd/sidecar/main.go`.
5. Add an example entry to `examples/config.yaml`.

That's it. No changes to the server, config loader, or any other package.

---

## Development

```bash
make build        # compile → ./bin/daimon
make run          # build + run with examples/config.yaml
make test         # go test ./...
make lint         # golangci-lint
make fmt          # gofmt + goimports
make license-check
```

---

## Roadmap

The current scope is intentionally narrow. Planned additions, in rough order:

- **Tool calls** via MCP
- **Memory / vector stores** as first-class components
- **Agent orchestration** primitives
- **Metrics** alongside traces (OTel)

The following are explicitly out of scope for now: gRPC, authentication, rate limiting, external plugin loading, pub/sub.

---

## License

Apache 2.0 — see [LICENSE](LICENSE).
