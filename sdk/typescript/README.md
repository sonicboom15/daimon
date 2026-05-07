# daimon-client

TypeScript / JavaScript client for [Daimon](https://github.com/sonicboom15/daimon) — a pluggable AI sidecar runtime.

## Installation

```sh
npm install daimon-client
```

Requires Node.js 18 or later. Works in ESM and CJS projects.

## Quick start

```typescript
import { Client } from 'daimon-client';

const client = new Client(); // default: http://127.0.0.1:3500

const reply = await client.chat('claude', 'What is a daimon?');
console.log(reply);
```

## Streaming

```typescript
for await (const text of client.stream('claude', 'Tell me a story.')) {
  process.stdout.write(text);
}
```

## Multi-turn conversations

Pass an array of messages to carry history yourself:

```typescript
const reply = await client.chat('claude', [
  { role: 'user',      content: 'My name is Alice.' },
  { role: 'assistant', content: 'Nice to meet you, Alice!' },
  { role: 'user',      content: 'What is my name?' },
]);
```

## Sessions

Let the sidecar maintain history server-side with a `session_id`:

```typescript
await client.chat('claude', 'My favourite colour is blue.', { session_id: 'chat-1' });
const reply = await client.chat('claude', 'What is my favourite colour?', { session_id: 'chat-1' });
// reply contains "blue"

await client.clearSession('chat-1');
```

## Inference parameters

All sampling parameters are optional and fall back to the component's configured defaults:

```typescript
const reply = await client.chat('gpt4o', 'Summarise this.', {
  model:       'gpt-4o',
  temperature: 0.2,
  max_tokens:  256,
  system:      'Be concise.',
});
```

## Vector store (memory)

Read and write documents in a configured vector store:

```typescript
const mem = client.memory('my-store');

// Upsert a document (returns the assigned ID)
const id = await mem.upsert('The Eiffel Tower is 330 metres tall.', {
  id: 'eiffel',
  metadata: { source: 'wikipedia' },
});

// Semantic search
const results = await mem.query('tall Paris structures', 5);
for (const r of results) {
  console.log(r.score.toFixed(3), r.content);
}

// Delete
await mem.delete('eiffel');
```

## Graph store

Interact with a configured graph database using Cypher:

```typescript
const kg = client.graph('knowledge-graph');

// Add nodes
await kg.addNode({ id: 'alice', labels: ['Person'], props: { name: 'Alice', age: 30 } });
await kg.addNode({ id: 'bob',   labels: ['Person'], props: { name: 'Bob' } });

// Add a relationship
await kg.addEdge('alice', 'bob', 'KNOWS', { props: { since: '2020' } });

// Run a Cypher query
const rows = await kg.cypher(
  'MATCH (a:Person)-[:KNOWS]->(b) RETURN a.name AS from, b.name AS to',
);
console.log(rows); // [{ from: 'Alice', to: 'Bob' }]

// Delete a node (and all its relationships)
await kg.deleteNode('alice');
```

## API reference

### `new Client(options?)`

| Option | Type | Default |
|---|---|---|
| `baseUrl` | `string` | `http://127.0.0.1:3500` |
| `timeout` | `number` (ms) | `120000` |

### `client.chat(component, prompt, options?)`

Returns the full response text as `Promise<string>`.

`prompt` can be a `string` or an array of `Message`-like objects.

### `client.stream(component, prompt, options?)`

Returns an `AsyncGenerator<string>` that yields text fragments as they arrive.

### `client.converse(component, options)`

Low-level method returning an `AsyncGenerator<Chunk>` for full control over chunk types (`text`, `tool_call`, `done`, `error`).

### `client.clearSession(sessionId)`

Deletes server-side session history. Returns `Promise<void>`. Safe to call on a non-existent session.

### `client.memory(store)` → `MemoryStoreClient`

Returns a client scoped to the named vector store.

| Method | Description |
|---|---|
| `upsert(content, options?)` | Insert or update a document. Returns the document ID. |
| `query(query, topK?)` | Semantic search. Returns `MemoryResult[]` sorted by descending score. |
| `delete(id)` | Delete a document by ID. |

### `client.graph(store)` → `GraphStoreClient`

Returns a client scoped to the named graph store.

| Method | Description |
|---|---|
| `addNode(options?)` | Add or update a node. Returns the node ID. |
| `addEdge(fromId, toId, relType, options?)` | Create a directed relationship. |
| `cypher(query, params?)` | Run a Cypher query. Returns `Record<string, unknown>[]`. |
| `deleteNode(id)` | Delete a node and all its relationships. |

### `ChatOptions` / `StreamOptions`

| Field | Type | Description |
|---|---|---|
| `model` | `string` | Override the component's default model |
| `system` | `string` | System prompt shorthand |
| `max_tokens` | `number` | |
| `temperature` | `number` | |
| `top_p` | `number` | |
| `top_k` | `number` | Anthropic only |
| `stop` | `string[]` | Stop sequences |
| `frequency_penalty` | `number` | |
| `presence_penalty` | `number` | |
| `seed` | `number` | |
| `session_id` | `string` | Server-side session ID |
| `onToolCall` | `(tc: ToolCall) => void` | Stream only — called when the model invokes a tool |

## License

Apache-2.0
