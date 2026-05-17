---
hide:
  - navigation
---

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
| `session_id` | string | no | Opaque identifier for a server-side session (see [Sessions](#sessions)). |

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

## Sessions

When `session_id` is included in a `/v1/converse` request, daimon maintains conversation history server-side so clients only need to send the new user turn instead of the full history.

**How it works:**

1. On the first request with a given `session_id`, daimon processes the request normally and stores the full exchange (incoming messages + final assistant response) under that ID.
2. On subsequent requests with the same `session_id`, daimon prepends the stored history to the incoming `messages` before calling the provider. The client only needs to supply the new user turn.
3. After each successful response, daimon appends the new turn (including any tool-call rounds) to the stored history.
4. Sessions are in-memory and lost when the sidecar process restarts.

**Example — two-turn conversation:**

```bash
# Turn 1: introduce yourself
curl -sN http://127.0.0.1:3500/v1/converse/claude \
  -H "Content-Type: application/json" \
  -d '{"session_id":"chat-1","messages":[{"role":"user","content":"My name is Alice."}]}'

# Turn 2: follow-up — server prepends [user:Alice, assistant:…] automatically
curl -sN http://127.0.0.1:3500/v1/converse/claude \
  -H "Content-Type: application/json" \
  -d '{"session_id":"chat-1","messages":[{"role":"user","content":"What is my name?"}]}'
```

**Python SDK:**

```python
client = daimon.Client()
client.llm("claude").chat("My name is Alice.", session_id="chat-1")
reply = client.llm("claude").chat("What is my name?", session_id="chat-1")
# reply == "Your name is Alice."

client.clear_session("chat-1")  # remove history when done
```

**TypeScript SDK:**

```typescript
const client = new Client();
await client.llm('claude').chat('My name is Alice.', { session_id: 'chat-1' });
const reply = await client.llm('claude').chat('What is my name?', { session_id: 'chat-1' });
await client.clearSession('chat-1');
```

---

## Memory Store API

Vector / document stores are exposed at `/v1/memory/{store}`. The `{store}` segment must match a component `name` declared in your config.

### `PUT /v1/memory/{store}/{id}` — Upsert with caller-supplied ID

```bash
curl -X PUT http://127.0.0.1:3500/v1/memory/docs/doc1 \
  -H "Content-Type: application/json" \
  -d '{"content": "The Eiffel Tower is 330 metres tall.", "metadata": {"source": "wikipedia"}}'
# {"id":"doc1"}
```

### `POST /v1/memory/{store}` — Upsert, server assigns ID

```bash
curl -X POST http://127.0.0.1:3500/v1/memory/docs \
  -H "Content-Type: application/json" \
  -d '{"content": "The Seine is the main river of Paris."}'
# {"id":"a1b2c3d4-..."}
```

**Request body:**

| Field | Type | Required | Description |
|---|---|---|---|
| `content` | string | **yes** | Document text. |
| `metadata` | object | no | Arbitrary string key/value pairs stored alongside the document. |

### `POST /v1/memory/{store}/query` — Semantic search

```bash
curl -X POST http://127.0.0.1:3500/v1/memory/docs/query \
  -H "Content-Type: application/json" \
  -d '{"query": "tall structures in Paris", "top_k": 3}'
```

```json
{
  "results": [
    { "id": "doc1", "content": "The Eiffel Tower is 330 metres tall.", "score": 0.91, "metadata": {"source": "wikipedia"} }
  ]
}
```

**Request body:**

| Field | Type | Required | Description |
|---|---|---|---|
| `query` | string | **yes** | Search text. |
| `top_k` | int | no | Maximum results to return (default: 5). |

### `DELETE /v1/memory/{store}/{id}` — Delete a document

Returns `204 No Content`. Idempotent.

```bash
curl -X DELETE http://127.0.0.1:3500/v1/memory/docs/doc1
```

!!! note
    Documents cannot be named `"query"`. The Go 1.22 router gives the literal path segment `/query` priority over `{id}` for the same method.

**Status codes for all memory endpoints:**

| Code | Meaning |
|---|---|
| `200 OK` | Success (upsert / query). |
| `204 No Content` | Success (delete). |
| `400 Bad Request` | Malformed JSON body. |
| `404 Not Found` | Unknown store name. |
| `500 Internal Server Error` | Backend error. |

**Python SDK:**

```python
store = client.memory("docs")
id = store.upsert("The Eiffel Tower is 330 metres tall.", id="doc1", metadata={"source": "wiki"})
results = store.query("tall Paris structures", top_k=3)
# results[0].id, results[0].content, results[0].score, results[0].metadata
store.delete("doc1")
```

**TypeScript SDK:**

```typescript
const store = client.memory('docs');
const id = await store.upsert('The Eiffel Tower is 330 metres tall.', { id: 'doc1', metadata: { source: 'wiki' } });
const results = await store.query('tall Paris structures', 3);
// results[0].id, results[0].content, results[0].score, results[0].metadata
await store.delete('doc1');
```

---

## Graph Store API

Graph stores are exposed at `/v1/graph/{store}`. The `{store}` segment must match a component `name` declared in your config.

### `PUT /v1/graph/{store}/nodes/{id}` — Add or update a node

```bash
curl -X PUT http://127.0.0.1:3500/v1/graph/kg/nodes/alice \
  -H "Content-Type: application/json" \
  -d '{"labels": ["Person"], "props": {"name": "Alice", "age": 30}}'
# {"id":"alice"}
```

### `POST /v1/graph/{store}/nodes` — Add a node, server assigns ID

```bash
curl -X POST http://127.0.0.1:3500/v1/graph/kg/nodes \
  -H "Content-Type: application/json" \
  -d '{"labels": ["Document"], "props": {"title": "Intro to Graphs"}}'
# {"id":"a1b2c3d4-..."}
```

**Request body:**

| Field | Type | Required | Description |
|---|---|---|---|
| `labels` | array of strings | no | Node labels (e.g. `["Person", "Employee"]`). |
| `props` | object | no | Arbitrary node properties. |

### `POST /v1/graph/{store}/edges` — Add a directed edge

```bash
curl -X POST http://127.0.0.1:3500/v1/graph/kg/edges \
  -H "Content-Type: application/json" \
  -d '{"from": "alice", "to": "bob", "type": "KNOWS", "props": {"since": "2020"}}'
```

Returns `204 No Content`.

**Request body:**

| Field | Type | Required | Description |
|---|---|---|---|
| `from` | string | **yes** | Source node ID. |
| `to` | string | **yes** | Target node ID. |
| `type` | string | **yes** | Relationship type (must match `^[A-Za-z_][A-Za-z0-9_]*$`). |
| `props` | object | no | Arbitrary relationship properties. |

### `POST /v1/graph/{store}/cypher` — Run a Cypher query

```bash
curl -X POST http://127.0.0.1:3500/v1/graph/kg/cypher \
  -H "Content-Type: application/json" \
  -d '{"query": "MATCH (a:Person)-[:KNOWS]->(b) RETURN a.name, b.name", "params": {}}'
```

```json
{ "rows": [{ "a.name": "Alice", "b.name": "Bob" }] }
```

**Request body:**

| Field | Type | Required | Description |
|---|---|---|---|
| `query` | string | **yes** | Cypher query string. |
| `params` | object | no | Named parameters (`$name` syntax in the query). |

### `DELETE /v1/graph/{store}/nodes/{id}` — Delete a node

Deletes the node and all its relationships. Returns `204 No Content`. Idempotent.

```bash
curl -X DELETE http://127.0.0.1:3500/v1/graph/kg/nodes/alice
```

**Status codes for all graph endpoints:** same as memory store endpoints above.

**Python SDK:**

```python
graph = client.graph("kg")
graph.add_node(id="alice", labels=["Person"], props={"name": "Alice"})
graph.add_edge("alice", "bob", "KNOWS", props={"since": "2020"})
rows = graph.cypher("MATCH (a)-[:KNOWS]->(b) RETURN a.name, b.name")
graph.delete_node("alice")
```

**TypeScript SDK:**

```typescript
const graph = client.graph('kg');
await graph.addNode({ id: 'alice', labels: ['Person'], props: { name: 'Alice' } });
await graph.addEdge('alice', 'bob', 'KNOWS', { props: { since: '2020' } });
const rows = await graph.cypher('MATCH (a)-[:KNOWS]->(b) RETURN a.name, b.name');
await graph.deleteNode('alice');
```

---

## `DELETE /v1/sessions/{id}`

Clears the stored conversation history for the given session ID. Returns `204 No Content`. Idempotent — deleting a session that does not exist is not an error.

```bash
curl -sX DELETE http://127.0.0.1:3500/v1/sessions/chat-1
```

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
