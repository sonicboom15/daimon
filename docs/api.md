# HTTP API

Daimon exposes two endpoints. All communication uses plain HTTP — no gRPC, no WebSockets.

Base URL: `http://127.0.0.1:3500` (default port; configurable in YAML).

---

## `POST /v1/converse/{component}`

Stream a chat request. The `{component}` path segment selects which provider handles the request — it must match a `name` defined in your config.

### Request

**Headers:**

```
Content-Type: application/json
```

**Body:**

```json
{
  "messages": [
    { "role": "system",    "content": "You are a helpful assistant." },
    { "role": "user",      "content": "What is a daimon?" }
  ],
  "model":              "gpt-4o-mini",
  "system":             "Shorthand for a system message — prepended to messages.",
  "max_tokens":         512,
  "temperature":        0.7,
  "top_p":              0.9,
  "top_k":              50,
  "stop":               ["Human:"],
  "frequency_penalty":  0.0,
  "presence_penalty":   0.0,
  "seed":               42,
  "tools": [
    {
      "name":         "get_weather",
      "description":  "Get the current weather for a city.",
      "input_schema": {
        "type": "object",
        "properties": {
          "city": { "type": "string", "description": "City name" }
        },
        "required": ["city"]
      }
    }
  ]
}
```

**Field reference:**

| Field | Type | Required | Description |
|---|---|---|---|
| `messages` | array | **yes** | Conversation history (see [Messages](#messages)) |
| `model` | string | no | Model name override. Falls back to the component's `default_model`. |
| `system` | string | no | Shorthand system prompt. Ignored if `messages` already contains a system role. |
| `max_tokens` | int | no | Maximum tokens to generate. |
| `temperature` | float | no | Sampling temperature. |
| `top_p` | float | no | Nucleus sampling. |
| `top_k` | int | no | Top-K sampling (Anthropic only). |
| `stop` | array of strings | no | Stop sequences. |
| `frequency_penalty` | float | no | Frequency penalty (OpenAI / llamacpp). |
| `presence_penalty` | float | no | Presence penalty (OpenAI / llamacpp). |
| `seed` | int | no | RNG seed (OpenAI / llamacpp). |
| `tools` | array | no | Tools the model may call (see [Tools](#tools)). MCP tools are injected automatically. |

Omitted inference parameters fall back to the component's configured `defaults:`, then to the provider's own defaults.

### Messages

Each message has a `role` and `content`:

| Role | When to use |
|---|---|
| `system` | Instructions for the model. Usually first. |
| `user` | A human turn. |
| `assistant` | A previous model response. Used for multi-turn. |
| `assistant` + `tool_calls` | A model turn where the model requested tool calls. |
| `tool` | The result of a tool call. Must follow the assistant turn that requested it. |

**Standard message:**
```json
{ "role": "user", "content": "Hello!" }
```

**Assistant message with tool call (for building history):**
```json
{
  "role": "assistant",
  "tool_calls": [
    { "id": "call_1", "name": "get_weather", "input": { "city": "London" } }
  ]
}
```

**Tool result:**
```json
{ "role": "tool", "content": "12°C, partly cloudy", "tool_call_id": "call_1" }
```

### Tools

Tools follow the JSON Schema format:

```json
{
  "name": "search_web",
  "description": "Search the web for up-to-date information.",
  "input_schema": {
    "type": "object",
    "properties": {
      "query": { "type": "string" }
    },
    "required": ["query"]
  }
}
```

!!! tip
    You don't need to define tools in the request if you've configured MCP servers — daimon injects them automatically and executes them for you.

### Response

**Headers:**

```
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive
```

The response body is a stream of Server-Sent Events. Each event is a JSON object on a `data:` line, terminated by a blank line:

```
data: {"type":"text","text":"In ancient Greek thought"}

data: {"type":"text","text":", a daimon is a guiding spirit."}

data: {"type":"done"}
```

**Chunk types:**

| `type` | Additional fields | Description |
|---|---|---|
| `text` | `text: string` | A text fragment from the model. Concatenate to build the full response. |
| `tool_call` | `tool_call: {id, name, input}` | The model invoked a tool. Daimon executes it automatically and continues. This event is forwarded so you can show progress in your UI. |
| `done` | — | The stream completed successfully. No further events follow. |
| `error` | `error: string` | A terminal error occurred. No further events follow. |

### Tool call event

```json
{
  "type": "tool_call",
  "tool_call": {
    "id":    "call_abc123",
    "name":  "get_weather",
    "input": { "city": "Tokyo" }
  }
}
```

After emitting this event, daimon executes the tool via the owning MCP server, appends the result to the conversation, and continues calling the model. The client does not need to take any action.

### HTTP status codes

| Code | Meaning |
|---|---|
| `200 OK` | Request accepted; SSE stream follows. |
| `400 Bad Request` | Invalid JSON body. |
| `404 Not Found` | Unknown component name. |
| `500 Internal Server Error` | Provider returned an error before streaming began. |

Errors that occur *during* streaming are emitted as `{"type":"error","error":"..."}` events (the HTTP status is already 200 at that point).

---

## `GET /healthz`

Returns `200 OK` with body `ok` when the sidecar is up and ready.

```bash
curl http://127.0.0.1:3500/healthz
# ok
```

Use this for liveness probes, `waitHealthy` loops, or `docker-compose` health checks.

---

## Examples

### Minimal chat request

```bash
curl -sN http://127.0.0.1:3500/v1/converse/claude \
  -H "Content-Type: application/json" \
  -d '{
    "messages": [{ "role": "user", "content": "What is 2+2?" }]
  }'
```

### With sampling parameters

```bash
curl -sN http://127.0.0.1:3500/v1/converse/gpt4o \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o",
    "temperature": 0.2,
    "max_tokens": 256,
    "messages": [
      { "role": "system", "content": "Answer concisely." },
      { "role": "user",   "content": "Explain SSE in one sentence." }
    ]
  }'
```

### Multi-turn conversation

```bash
curl -sN http://127.0.0.1:3500/v1/converse/claude \
  -H "Content-Type: application/json" \
  -d '{
    "messages": [
      { "role": "user",      "content": "My name is Alice." },
      { "role": "assistant", "content": "Nice to meet you, Alice!" },
      { "role": "user",      "content": "What is my name?" }
    ]
  }'
```
