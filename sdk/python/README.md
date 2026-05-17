# daimon-client

Python client for [Daimon](https://github.com/sonicboom15/daimon) — a pluggable AI sidecar runtime.

## Installation

```bash
pip install daimon-client
```

## Quick start

```python
from daimon_client import Client

with Client(base_url="http://localhost:3500") as c:
    # Single LLM configured — no name needed
    reply = c.llm().chat("What is a daimon?")
    print(reply)

    # Multiple LLMs configured — pick by name
    reply = c.llm("claude").chat("What is a daimon?")
```

### Async

```python
import asyncio
from daimon_client import AsyncClient

async def main():
    async with AsyncClient(base_url="http://localhost:3500") as c:
        reply = await c.llm().chat("What is a daimon?")
        print(reply)

asyncio.run(main())
```

## Streaming

```python
with Client() as c:
    for text in c.llm().stream("Tell me a story."):
        print(text, end="", flush=True)
```

## Multi-turn conversations

Pass a list of messages to carry history yourself:

```python
reply = c.llm().chat([
    {"role": "user",      "content": "My name is Alice."},
    {"role": "assistant", "content": "Nice to meet you, Alice!"},
    {"role": "user",      "content": "What is my name?"},
])
```

## Sessions

Let the sidecar maintain history server-side with a `session_id`:

```python
llm = c.llm()
llm.chat("My favourite colour is blue.", session_id="chat-1")
reply = llm.chat("What is my favourite colour?", session_id="chat-1")
# reply contains "blue"

llm.clear_session("chat-1")
```

## Inference parameters

All sampling parameters are optional and fall back to the component's configured defaults:

```python
reply = c.llm("gpt4o").chat("Summarise this.", model="gpt-4o", temperature=0.2, max_tokens=256)
```

## Vector store (memory)

Read and write documents in a configured vector store:

```python
mem = c.memory("my-store")

# Upsert a document (returns the assigned ID)
doc_id = mem.upsert("The Eiffel Tower is 330 metres tall.", id="eiffel", metadata={"source": "wikipedia"})

# Semantic search
results = mem.query("tall Paris structures", top_k=3)
for r in results:
    print(f"{r.score:.3f}  {r.content}")

# Delete
mem.delete("eiffel")
```

### Async vector store

```python
async with AsyncClient() as c:
    mem = c.memory("my-store")
    await mem.upsert("Some content")
    results = await mem.query("my query")
```

## Graph store

Interact with a configured graph database using Cypher:

```python
kg = c.graph("knowledge-graph")

# Add nodes
kg.add_node(id="alice", labels=["Person"], props={"name": "Alice", "age": 30})
kg.add_node(id="bob",   labels=["Person"], props={"name": "Bob"})

# Add a relationship
kg.add_edge("alice", "bob", "KNOWS", props={"since": "2020"})

# Run a Cypher query
rows = kg.cypher(
    "MATCH (a:Person)-[:KNOWS]->(b) RETURN a.name AS from, b.name AS to"
)
print(rows)  # [{"from": "Alice", "to": "Bob"}]

# Delete a node (and all its relationships)
kg.delete_node("alice")
```

## API reference

### `Client(base_url?, timeout?)`

| Parameter | Default |
|---|---|
| `base_url` | `http://127.0.0.1:3500` |
| `timeout` | `120.0` seconds |

Use as a context manager (`with Client() as c`) or call `c.close()` manually.

### `c.llm(component="default")` → `LLMClient`

Returns a client scoped to the named LLM component. Omit `component` to use whichever single LLM is configured.

| Method | Description |
|---|---|
| `chat(prompt, **kwargs)` → `str` | Send and return the full text response. |
| `stream(prompt, **kwargs)` → `Iterator[str]` | Yield text fragments as they arrive. |
| `converse(*, messages, **kwargs)` → `Iterator[Chunk]` | Raw chunk stream for full control. |
| `clear_session(session_id)` | Delete server-side session history. |

`prompt` can be a `str` or a list of `{"role": ..., "content": ...}` dicts.

### Shorthand methods on `Client`

`c.chat(component, prompt, **kwargs)`, `c.stream(...)`, `c.converse(...)`, and `c.clear_session(...)` are convenience wrappers that call `c.llm(component).*`. They exist for quick scripts; prefer the `llm()` accessor for anything beyond a one-liner.

`AsyncClient` exposes the same API with `async def` methods and `AsyncLLMClient` via `c.llm()`.

### `c.memory(store="default")` → `MemoryStoreClient`

Returns a client scoped to the named vector store.

| Method | Description |
|---|---|
| `upsert(content, *, id?, metadata?)` | Insert or update a document. Returns the document ID. |
| `query(query, top_k=5)` | Semantic search. Returns `list[MemoryResult]` sorted by descending score. |
| `delete(id)` | Delete a document by ID. |

### `c.graph(store)` → `GraphStoreClient`

Returns a client scoped to the named graph store.

| Method | Description |
|---|---|
| `add_node(*, id?, labels?, props?)` | Add or update a node. Returns the node ID. |
| `add_edge(from_id, to_id, rel_type, *, props?)` | Create a directed relationship. |
| `cypher(query, params?)` | Run a Cypher query. Returns `list[dict]`. |
| `delete_node(id)` | Delete a node and all its relationships. |

### Keyword arguments for `chat` / `stream`

| Argument | Description |
|---|---|
| `model` | Override the component's default model |
| `system` | System prompt shorthand |
| `max_tokens` | |
| `temperature` | |
| `top_p` | |
| `top_k` | Anthropic only |
| `stop` | List of stop sequences |
| `frequency_penalty` | |
| `presence_penalty` | |
| `seed` | |
| `session_id` | Server-side session ID |

`AsyncClient` mirrors `Client` with `async def` methods and `async for` streaming.

## Links

- [Daimon on GitHub](https://github.com/sonicboom15/daimon)
- [Bug reports](https://github.com/sonicboom15/daimon/issues)

## License

Apache-2.0
