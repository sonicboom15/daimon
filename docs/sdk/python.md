# Python SDK

The `daimon-client` package provides sync and async clients for the daimon sidecar.

## Install

```bash
pip install daimon-client
```

**Requirements:** Python 3.11+, `httpx>=0.27`

---

## Quick example

```python
import daimon_client as daimon

with daimon.Client() as client:
    reply = client.llm('claude').chat('What is a daimon?')
    print(reply)
```

---

## Clients

### `Client` — synchronous

```python
client = daimon.Client(
    base_url="http://127.0.0.1:3500",  # default
    timeout=120.0,                      # seconds; default 120
)
```

Use as a context manager to reuse the HTTP connection across calls:

```python
with daimon.Client() as client:
    reply1 = client.llm('claude').chat('Hello')
    reply2 = client.llm('gpt4o').chat('World')
```

Without a context manager, a new connection is created per call.

### `AsyncClient` — asynchronous

```python
async with daimon.AsyncClient() as client:
    async for text in client.llm('claude').stream('Hello'):
        print(text, end="", flush=True)
```

---

## `LLMClient`

`client.llm(component)` returns an `LLMClient` scoped to the named component. All LLM
methods live here. Omit the component name to use whichever single LLM is configured.

```python
llm = client.llm('claude')   # or client.llm() if only one LLM is configured
reply = llm.chat('What is a daimon?')
```

### `chat()` — full response string

Collects all text chunks and returns the complete response as a single string.

```python
reply = client.llm('gpt4o').chat('What is the capital of France?')
print(reply)  # "The capital of France is Paris."
```

`prompt` can be a plain string or a list of message objects:

```python
reply = client.llm('claude').chat([
    {'role': 'user',      'content': 'My name is Alice.'},
    {'role': 'assistant', 'content': 'Nice to meet you, Alice!'},
    {'role': 'user',      'content': 'What is my name?'},
])
```

### `stream()` — text fragments

Yields `str` fragments as they arrive.

```python
for text in client.llm('claude').stream('Tell me a story.'):
    print(text, end="", flush=True)
```

**Observing tool calls:**

```python
def log_tool(tc: daimon.ToolCall) -> None:
    print(f"\n[calling: {tc.name}({tc.input})]")

for text in client.llm('claude').stream(
    "What's the weather in Tokyo?",
    on_tool_call=log_tool,
):
    print(text, end="", flush=True)
```

| Parameter | Type | Description |
|---|---|---|
| `prompt_or_messages` | `str` \| `list[Message \| dict]` | A plain string or full message history |
| `on_tool_call` | `Callable[[ToolCall], None]` | Called for each tool call the model makes |
| `session_id` | `str` | Server-side session ID (see [Sessions](#sessions)) |
| `model` | `str` | Model override |
| `**kwargs` | | Any inference parameter (see below) |

Raises `DaimonError` on a stream error chunk.

### `converse()` — raw chunk stream

Yields `Chunk` objects for full control over chunk types.

```python
for chunk in client.llm('claude').converse(
    messages=[{'role': 'user', 'content': 'Hello'}]
):
    if chunk.type == 'text':
        print(chunk.text, end="", flush=True)
    elif chunk.type == 'tool_call':
        print(f"\n[tool: {chunk.tool_call.name}]")
    elif chunk.type == 'error':
        raise RuntimeError(chunk.error)
```

### `clear_session(session_id)`

Deletes server-side session history. Returns `None`. Safe to call on a session that does not exist.

```python
client.llm().clear_session('chat-1')
```

---

## Shorthand methods on `Client`

`client.chat(component, prompt, **kwargs)`, `client.stream(...)`, `client.converse(...)`,
and `client.clear_session(...)` are convenience wrappers that call `client.llm(component).*`.
They exist for quick scripts; prefer the `llm()` accessor for anything beyond a one-liner.

---

## Sessions

Pass `session_id` to any call and daimon maintains conversation history server-side. You
only need to send the new user turn — no need to replay the full history from the client.

```python
llm = client.llm('claude')

# Turn 1 — introduce a fact
llm.chat('My name is Alice.', session_id='chat-1')

# Turn 2 — server prepends the stored history automatically
reply = llm.chat('What is my name?', session_id='chat-1')
# reply: "Your name is Alice."

# Clean up when done
llm.clear_session('chat-1')
```

`session_id` works with both `chat()` and `stream()`:

```python
# stream() turn 1
list(client.llm('claude').stream('My favourite colour is blue.', session_id='s1'))

# chat() turn 2 — server remembers the colour
reply = client.llm('claude').chat('What is my favourite colour?', session_id='s1')
```

Sessions are in-memory and cleared when the sidecar restarts.

---

## Inference parameters

All inference parameters are optional keyword arguments on `stream`, `chat`, and `converse`.
They override any defaults set in the server config.

| Parameter | Type | Providers | Description |
|---|---|---|---|
| `model` | `str` | all | Model name override |
| `system` | `str` | all | System prompt (shorthand) |
| `max_tokens` | `int` | all | Maximum tokens to generate |
| `temperature` | `float` | all | Sampling temperature |
| `top_p` | `float` | all | Nucleus sampling |
| `top_k` | `int` | Anthropic | Top-K sampling |
| `stop` | `list[str]` | all | Stop sequences |
| `frequency_penalty` | `float` | OpenAI, llamacpp | Frequency penalty |
| `presence_penalty` | `float` | OpenAI, llamacpp | Presence penalty |
| `seed` | `int` | OpenAI, llamacpp | RNG seed |

```python
reply = client.llm('gpt4o').chat(
    'Write a haiku about Python.',
    model='gpt-4o',
    temperature=0.9,
    max_tokens=64,
    stop=['\n\n'],
)
```

---

## Types

### `Message`

```python
@dataclass
class Message:
    role: Literal["system", "user", "assistant", "tool"]
    content: str = ""
    tool_calls: list[ToolCall] = field(default_factory=list)
    tool_call_id: str = ""
```

```python
messages = [
    daimon.Message(role='system',    content='You are a helpful assistant.'),
    daimon.Message(role='user',      content='Hello!'),
    daimon.Message(role='assistant', content='Hi! How can I help you today?'),
    daimon.Message(role='user',      content='What is 2+2?'),
]
reply = client.llm('claude').chat(messages)
```

Plain dicts also work anywhere a `Message` is expected:

```python
messages = [{'role': 'user', 'content': 'Hello!'}]
```

### `Tool`

```python
@dataclass
class Tool:
    name: str
    description: str = ""
    input_schema: dict[str, Any] = field(default_factory=lambda: {"type": "object"})
```

```python
weather_tool = daimon.Tool(
    name='get_weather',
    description='Get the current weather for a city.',
    input_schema={
        'type': 'object',
        'properties': {'city': {'type': 'string'}},
        'required': ['city'],
    },
)

for text in client.llm('claude').stream('Weather in Tokyo?', tools=[weather_tool]):
    print(text, end="", flush=True)
```

### `ToolCall`

```python
@dataclass
class ToolCall:
    id: str
    name: str
    input: dict[str, Any]
```

Received via `on_tool_call` callback in `stream()`, or as `chunk.tool_call` in `converse()`.

### `Chunk`

```python
@dataclass
class Chunk:
    type: Literal["text", "tool_call", "error", "done"]
    text: str = ""
    tool_call: ToolCall | None = None
    error: str = ""
```

### `DaimonError`

Raised by `stream()` and `chat()` when the server emits an error chunk or returns a non-2xx status.

```python
try:
    reply = client.llm('claude').chat('Hello')
except daimon.DaimonError as e:
    print(f'Error: {e}')
```

---

## Examples

### Multi-turn conversation

```python
import daimon_client as daimon

messages: list[daimon.Message] = [
    daimon.Message(role='system', content='You are a helpful assistant.'),
]

with daimon.Client() as client:
    llm = client.llm('claude')
    while True:
        user_input = input('You: ')
        messages.append(daimon.Message(role='user', content=user_input))

        print('Claude: ', end='', flush=True)
        reply = ''
        for text in llm.stream(messages):
            print(text, end='', flush=True)
            reply += text
        print()

        messages.append(daimon.Message(role='assistant', content=reply))
```

### Async streaming

```python
import asyncio
import daimon_client as daimon

async def main():
    async with daimon.AsyncClient() as client:
        async for text in client.llm('gpt4o').stream(
            'Explain async/await in Python.',
            temperature=0.5,
            max_tokens=200,
        ):
            print(text, end='', flush=True)
    print()

asyncio.run(main())
```

### Structured output via `converse()`

```python
import json, daimon_client as daimon

extractor = daimon.Tool(
    name='extract_entities',
    description='Extract named entities from text.',
    input_schema={
        'type': 'object',
        'properties': {
            'people': {'type': 'array', 'items': {'type': 'string'}},
            'places': {'type': 'array', 'items': {'type': 'string'}},
        },
    },
)

with daimon.Client() as client:
    for chunk in client.llm('claude').converse(
        messages=[{'role': 'user', 'content': 'Alice met Bob in Paris last Tuesday.'}],
        tools=[extractor],
    ):
        if chunk.type == 'tool_call':
            print(json.dumps(chunk.tool_call.input, indent=2))
```
