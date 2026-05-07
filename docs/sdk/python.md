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
    for text in client.stream("claude", "What is a daimon?"):
        print(text, end="", flush=True)
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
    reply1 = client.chat("claude", "Hello")
    reply2 = client.chat("gpt4o", "World")
```

Without a context manager, a new connection is created per call.

### `AsyncClient` — asynchronous

```python
async with daimon.AsyncClient() as client:
    async for text in client.stream("claude", "Hello"):
        print(text, end="", flush=True)
```

Both clients expose the same three methods: `stream`, `chat`, and `converse`.

---

## Methods

### `stream()` — text fragments

Yields `str` fragments as they arrive. The simplest way to print a streaming response.

```python
for text in client.stream(component, prompt_or_messages, **kwargs):
    print(text, end="", flush=True)
```

| Parameter | Type | Description |
|---|---|---|
| `component` | `str` | Component name from config (e.g. `"claude"`) |
| `prompt_or_messages` | `str` \| `list[Message \| dict]` | A plain string or full message history |
| `on_tool_call` | `Callable[[ToolCall], None]` | Called for each tool call the model makes |
| `session_id` | `str` | Server-side session ID (see [Sessions](#sessions)) |
| `model` | `str` | Model override |
| `**kwargs` | | Any inference parameter (see below) |

Raises `DaimonError` on a stream error chunk.

**Observing tool calls without handling them:**

```python
def log_tool(tc: daimon.ToolCall) -> None:
    print(f"\n[calling: {tc.name}({tc.input})]")

for text in client.stream("claude", "What's the weather in Tokyo?", on_tool_call=log_tool):
    print(text, end="", flush=True)
```

### `chat()` — full response string

Collects all text chunks and returns the complete response as a single string. Convenient for non-interactive use.

```python
reply = client.chat(component, prompt_or_messages, **kwargs)
```

```python
answer = client.chat("gpt4o", "What is the capital of France?")
print(answer)  # "The capital of France is Paris."
```

### `converse()` — raw chunk stream

Yields `Chunk` objects. Use this when you need full control — access to tool call metadata, error details, or the done signal.

```python
for chunk in client.converse(component, messages=messages, **kwargs):
    if chunk.type == "text":
        print(chunk.text, end="", flush=True)
    elif chunk.type == "tool_call":
        print(f"\n[tool: {chunk.tool_call.name}]")
    elif chunk.type == "error":
        raise RuntimeError(chunk.error)
```

---

## Sessions

Pass `session_id` to any call and daimon maintains conversation history server-side. You only need to send the new user turn — no need to replay the full history from the client.

```python
client = daimon.Client()

# Turn 1 — introduce a fact
client.chat("claude", "My name is Alice.", session_id="chat-1")

# Turn 2 — server prepends the stored history automatically
reply = client.chat("claude", "What is my name?", session_id="chat-1")
# reply: "Your name is Alice."

# Clean up when done
client.clear_session("chat-1")
```

`session_id` works with both `chat()` and `stream()`:

```python
# stream() turn 1
list(client.stream("claude", "My favourite colour is blue.", session_id="s1"))

# chat() turn 2 — server remembers the colour
reply = client.chat("claude", "What is my favourite colour?", session_id="s1")
```

### `clear_session(session_id)`

Deletes server-side session history. Returns `None`. Safe to call on a session that does not exist.

```python
client.clear_session("chat-1")
```

Async equivalent:

```python
await async_client.clear_session("chat-1")
```

Sessions are in-memory and cleared when the sidecar restarts.

---

## Inference parameters

All inference parameters are optional keyword arguments on `stream`, `chat`, and `converse`. They override any defaults set in the server config.

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
reply = client.chat(
    "gpt4o",
    "Write a haiku about Python.",
    model="gpt-4o",
    temperature=0.9,
    max_tokens=64,
    stop=["\n\n"],
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
    daimon.Message(role="system",    content="You are a helpful assistant."),
    daimon.Message(role="user",      content="Hello!"),
    daimon.Message(role="assistant", content="Hi! How can I help you today?"),
    daimon.Message(role="user",      content="What is 2+2?"),
]
reply = client.chat("claude", messages)
```

Plain dicts also work anywhere a `Message` is expected:

```python
messages = [
    {"role": "user", "content": "Hello!"},
]
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
    name="get_weather",
    description="Get the current weather for a city.",
    input_schema={
        "type": "object",
        "properties": {"city": {"type": "string"}},
        "required": ["city"],
    },
)

for text in client.stream("claude", "Weather in Tokyo?", tools=[weather_tool]):
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

Raised by `stream()` when the server emits an error chunk.

```python
try:
    for text in client.stream("claude", "Hello"):
        print(text, end="")
except daimon.DaimonError as e:
    print(f"Error: {e}")
```

---

## Examples

### Multi-turn conversation

```python
import daimon_client as daimon

messages: list[daimon.Message] = [
    daimon.Message(role="system", content="You are a helpful assistant."),
]

with daimon.Client() as client:
    while True:
        user_input = input("You: ")
        messages.append(daimon.Message(role="user", content=user_input))

        print("Claude: ", end="", flush=True)
        reply = ""
        for text in client.stream("claude", messages):
            print(text, end="", flush=True)
            reply += text
        print()

        messages.append(daimon.Message(role="assistant", content=reply))
```

### Async streaming

```python
import asyncio
import daimon_client as daimon

async def main():
    async with daimon.AsyncClient() as client:
        async for text in client.stream(
            "gpt4o",
            "Explain async/await in Python.",
            temperature=0.5,
            max_tokens=200,
        ):
            print(text, end="", flush=True)
    print()

asyncio.run(main())
```

### Structured output via low-level `converse`

```python
import json, daimon_client as daimon

extractor = daimon.Tool(
    name="extract_entities",
    description="Extract named entities from text.",
    input_schema={
        "type": "object",
        "properties": {
            "people": {"type": "array", "items": {"type": "string"}},
            "places": {"type": "array", "items": {"type": "string"}},
        },
    },
)

with daimon.Client() as client:
    for chunk in client.converse(
        "claude",
        messages=[{"role": "user", "content": "Alice met Bob in Paris last Tuesday."}],
        tools=[extractor],
    ):
        if chunk.type == "tool_call":
            print(json.dumps(chunk.tool_call.input, indent=2))
```
