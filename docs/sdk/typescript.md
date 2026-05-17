# TypeScript SDK

The `daimon-client` package provides a TypeScript / JavaScript client for the daimon sidecar. It works in Node.js 18+ and any runtime that supports the native `fetch` API.

## Install

```sh
npm install daimon-client
```

**Requirements:** Node.js 18+. No runtime dependencies.

---

## Quick example

```typescript
import { Client } from 'daimon-client';

const client = new Client(); // default: http://127.0.0.1:3500

const reply = await client.llm('claude').chat('What is a daimon?');
console.log(reply);
```

---

## Clients

### `new Client(options?)`

```typescript
const client = new Client({
  baseUrl: 'http://127.0.0.1:3500', // default
  timeout: 120_000,                  // ms; default 120 000
});
```

---

## Methods

### `chat()` — full response string

Collects all text chunks and returns the complete response as a `Promise<string>`.

```typescript
const reply = await client.llm('gpt4o').chat('What is the capital of France?');
console.log(reply); // "The capital of France is Paris."
```

`prompt` can be a plain string or an array of message objects:

```typescript
const reply = await client.llm('claude').chat([
  { role: 'user',      content: 'My name is Alice.' },
  { role: 'assistant', content: 'Nice to meet you, Alice!' },
  { role: 'user',      content: 'What is my name?' },
]);
```

### `stream()` — text fragments

Returns an `AsyncGenerator<string>` that yields text fragments as they arrive.

```typescript
for await (const text of client.llm('claude').stream('Tell me a story.')) {
  process.stdout.write(text);
}
```

**Observing tool calls:**

```typescript
for await (const text of client.llm('claude').stream("What's the weather in Tokyo?", {
  onToolCall: (tc) => console.log(`[tool: ${tc.name}]`),
})) {
  process.stdout.write(text);
}
```

### `converse()` — raw chunk stream

Returns an `AsyncGenerator<Chunk>` for full control over chunk types.

```typescript
for await (const chunk of client.llm('claude').converse({ messages: [{ role: 'user', content: 'Hello' }] })) {
  if (chunk.type === 'text')       process.stdout.write(chunk.text);
  if (chunk.type === 'tool_call')  console.log('[tool]', chunk.toolCall?.name);
  if (chunk.type === 'error')      throw new Error(chunk.error);
}
```

### `clearSession()` — delete session history

Deletes server-side session history. Returns `Promise<void>`. Safe to call on a session that does not exist.

```typescript
await client.llm().clearSession('chat-1');
```

---

## Sessions

Pass `session_id` in any call and the sidecar maintains conversation history server-side. You only need to send the new user turn — no need to replay the full history from the client.

```typescript
// Turn 1 — introduce a fact
await client.llm('claude').chat('My name is Alice.', { session_id: 'chat-1' });

// Turn 2 — server prepends the stored history automatically
const reply = await client.llm('claude').chat('What is my name?', { session_id: 'chat-1' });
// reply: "Your name is Alice."

// Clean up when done
await client.llm().clearSession('chat-1');
```

`session_id` is supported on both `chat()` and `stream()`:

```typescript
// stream() turn 1
for await (const text of client.llm('claude').stream('My favourite colour is blue.', { session_id: 's1' })) {
  process.stdout.write(text);
}

// chat() turn 2 — server remembers the colour
const reply = await client.llm('claude').chat('What is my favourite colour?', { session_id: 's1' });
```

Sessions are in-memory and cleared when the sidecar restarts.

---

## Inference parameters

All parameters are optional and fall back to the component's configured defaults.

| Parameter | Type | Providers | Description |
|---|---|---|---|
| `model` | `string` | all | Model name override |
| `system` | `string` | all | System prompt shorthand |
| `max_tokens` | `number` | all | Maximum tokens to generate |
| `temperature` | `number` | all | Sampling temperature |
| `top_p` | `number` | all | Nucleus sampling |
| `top_k` | `number` | Anthropic | Top-K sampling |
| `stop` | `string[]` | all | Stop sequences |
| `frequency_penalty` | `number` | OpenAI, llamacpp | |
| `presence_penalty` | `number` | OpenAI, llamacpp | |
| `seed` | `number` | OpenAI, llamacpp | RNG seed |
| `session_id` | `string` | all | Server-side session ID |
| `onToolCall` | `(tc: ToolCall) => void` | stream only | Called when the model invokes a tool |

```typescript
const reply = await client.llm('gpt4o').chat('Write a haiku about Go.', {
  model:       'gpt-4o',
  temperature: 0.9,
  max_tokens:  64,
});
```

---

## Types

### `Message`

```typescript
interface Message {
  role:         'system' | 'user' | 'assistant' | 'tool';
  content?:     string;
  tool_calls?:  ToolCall[];
  tool_call_id?: string;
}
```

Plain objects are accepted anywhere a `Message` is expected.

### `ToolCall`

```typescript
class ToolCall {
  id:    string;
  name:  string;
  input: Record<string, unknown>;
}
```

### `Chunk`

```typescript
class Chunk {
  type:      'text' | 'tool_call' | 'error' | 'done';
  text:      string;
  toolCall?: ToolCall;
  error:     string;
}
```

### `DaimonError`

Thrown by `chat()` and `stream()` when the server emits an error chunk or returns a non-2xx status.

```typescript
try {
  const reply = await client.llm('claude').chat('Hello');
} catch (e) {
  if (e instanceof DaimonError) console.error('daimon error:', e.message);
}
```

---

## Examples

### Multi-turn chat loop

```typescript
import * as readline from 'node:readline/promises';
import { Client } from 'daimon-client';

const client = new Client();
const rl = readline.createInterface({ input: process.stdin, output: process.stdout });
const SESSION = 'repl-session';

while (true) {
  const input = await rl.question('You: ');
  process.stdout.write('Claude: ');
  for await (const text of client.llm('claude').stream(input, { session_id: SESSION })) {
    process.stdout.write(text);
  }
  console.log();
}
```

### Streaming with inference parameters

```typescript
for await (const text of client.llm('gpt4o').stream('Explain async/await.', {
  model:       'gpt-4o',
  temperature: 0.5,
  max_tokens:  200,
  system:      'Be concise.',
})) {
  process.stdout.write(text);
}
```

### Structured output via `converse()`

```typescript
import { Client, Tool } from 'daimon-client';

const client = new Client();
const extractor = new Tool(
  'extract_entities',
  'Extract named entities from text.',
  {
    type: 'object',
    properties: {
      people: { type: 'array', items: { type: 'string' } },
      places: { type: 'array', items: { type: 'string' } },
    },
  },
);

for await (const chunk of client.llm('claude').converse({
  messages: [{ role: 'user', content: 'Alice met Bob in Paris last Tuesday.' }],
  tools:    [extractor],
})) {
  if (chunk.type === 'tool_call') {
    console.log(JSON.stringify(chunk.toolCall?.input, null, 2));
  }
}
```
