# Java SDK

The `daimon-client` Java library provides a synchronous client for the daimon sidecar. It runs on any JVM that supports Java 17+.

## Install

=== "Gradle"

    ```groovy
    dependencies {
        implementation 'io.github.sonicboom15:daimon-client:0.4.0'
    }
    ```

=== "Maven"

    ```xml
    <dependency>
        <groupId>io.github.sonicboom15</groupId>
        <artifactId>daimon-client</artifactId>
        <version>0.4.0</version>
    </dependency>
    ```

**Requirements:** Java 17+. One runtime dependency: [Gson](https://github.com/google/gson).

---

## Quick example

```java
import io.github.sonicboom15.daimon.Client;

Client client = new Client();   // default: http://127.0.0.1:3500

String reply = client.llm("claude").chat("What is a daimon?");
System.out.println(reply);
```

---

## `Client`

```java
// Default: http://127.0.0.1:3500, 120 s timeout
Client client = new Client();

// Custom base URL
Client client = new Client("http://127.0.0.1:3500");

// Custom base URL + timeout (milliseconds)
Client client = new Client("http://127.0.0.1:3500", 60_000L);
```

`Client` is thread-safe and designed to be reused for the lifetime of the application.

---

## `LLMClient`

`client.llm(component)` returns an `LLMClient` scoped to the named component.

```java
LLMClient llm = client.llm("claude");   // scoped to "claude" component
LLMClient llm = client.llm();           // scoped to "default" component
```

---

## Methods

### `chat()` — full response string

Blocks until the full response is received, then returns it as a `String`.

```java
String reply = client.llm("claude").chat("What is the capital of France?");
// "The capital of France is Paris."
```

Pass a `ChatOptions` for inference parameters:

```java
ChatOptions opts = ChatOptions.builder()
        .model("claude-opus-4-7")
        .temperature(0.7)
        .maxTokens(512)
        .system("Be concise.")
        .build();

String reply = client.llm("claude").chat("Explain recursion.", opts);
```

Pass a full message history:

```java
List<Message> messages = List.of(
    Message.user("My name is Alice."),
    Message.assistant("Nice to meet you, Alice!"),
    Message.user("What is my name?")
);
String reply = client.llm("claude").chat(messages, ChatOptions.defaults());
```

### `stream()` — text fragments

Returns an `Iterable<String>` that lazily yields text fragments as they arrive over SSE.

```java
for (String text : client.llm("claude").stream("Tell me a story.")) {
    System.out.print(text);
    System.out.flush();
}
```

With options:

```java
ChatOptions opts = ChatOptions.builder().temperature(0.9).build();
for (String text : client.llm("gpt4o").stream("Write a poem.", opts)) {
    System.out.print(text);
}
```

### `converse()` — raw chunk stream

Returns an `Iterable<Chunk>` for full control over chunk types (text, tool calls, errors).

```java
List<Message> messages = List.of(Message.user("Hello"));
for (Chunk chunk : client.llm("claude").converse(messages, ChatOptions.defaults())) {
    if (chunk.isText())     System.out.print(chunk.text());
    if (chunk.isToolCall()) System.out.println("[tool] " + chunk.toolCall().name());
    if (chunk.isError())    throw new RuntimeException(chunk.error());
    if (chunk.isDone())     break;
}
```

### `clearSession()` — delete session history

Deletes server-side session history. Safe to call on a session that does not exist.

```java
client.llm("claude").clearSession("chat-1");
```

---

## Sessions

Pass a `session_id` via `ChatOptions` and the sidecar maintains conversation history server-side. You only need to send the new user turn each time.

```java
ChatOptions turn1 = ChatOptions.builder().sessionId("chat-1").build();
client.llm("claude").chat("My name is Alice.", turn1);

// Server prepends the stored history automatically.
String reply = client.llm("claude").chat("What is my name?", turn1);
// "Your name is Alice."

client.llm("claude").clearSession("chat-1");
```

`session_id` works with both `chat()` and `stream()`.

---

## Inference parameters

All parameters are optional. Request values override the component's configured defaults.

| Parameter | Type | Providers | Description |
|---|---|---|---|
| `model` | `String` | all | Model name override |
| `system` | `String` | all | System prompt |
| `sessionId` | `String` | all | Server-side session ID |
| `maxTokens` | `Integer` | all | Maximum tokens to generate |
| `temperature` | `Double` | all | Sampling temperature |
| `topP` | `Double` | all | Nucleus sampling |
| `topK` | `Integer` | Anthropic, Gemini | Top-K sampling |
| `seed` | `Integer` | OpenAI, llamacpp, Mistral | RNG seed |
| `frequencyPenalty` | `Double` | OpenAI, llamacpp, Mistral | Frequency penalty |
| `presencePenalty` | `Double` | OpenAI, llamacpp, Mistral | Presence penalty |
| `tools` | `List<Tool>` | all | Tools the model may call |

---

## Types

### `Message`

```java
Message.user("Hello")                      // role: user
Message.assistant("Hi there!")             // role: assistant
Message.system("You are a pirate.")        // role: system
new Message("tool", content, toolCallId)   // role: tool
```

### `ChatOptions`

```java
ChatOptions opts = ChatOptions.builder()
        .model("claude-sonnet-4-6")
        .system("Be concise.")
        .sessionId("my-session")
        .maxTokens(256)
        .temperature(0.5)
        .topP(0.9)
        .build();
```

`ChatOptions.defaults()` returns an instance with all fields `null` (provider defaults apply).

### `Chunk`

```java
chunk.isText()        // type == "text"
chunk.isToolCall()    // type == "tool_call"
chunk.isError()       // type == "error"
chunk.isDone()        // type == "done"

chunk.text()          // text content (when isText())
chunk.toolCall()      // ToolCall object (when isToolCall())
chunk.error()         // error message string (when isError())
```

### `ToolCall`

```java
chunk.toolCall().id()     // call ID
chunk.toolCall().name()   // function name
chunk.toolCall().input()  // JsonObject of arguments
```

### `DaimonException`

Thrown by `chat()`, `stream()`, and `converse()` when the server emits an error chunk or returns a non-2xx HTTP status.

```java
try {
    String reply = client.llm("claude").chat("Hello");
} catch (DaimonException e) {
    System.err.println("daimon error: " + e.getMessage());
}
```

---

## Shorthand methods

`Client` exposes shorthand helpers that skip creating an `LLMClient` explicitly:

```java
// Equivalent to client.llm("claude").chat("Hello")
String reply = client.chat("claude", "Hello");

// Equivalent to client.llm("claude").stream("Hello")
Iterable<String> stream = client.stream("claude", "Hello");
```

---

## Examples

### Multi-turn chat loop

```java
import io.github.sonicboom15.daimon.*;
import java.util.Scanner;

Client client = new Client();
Scanner scanner = new Scanner(System.in);
ChatOptions opts = ChatOptions.builder().sessionId("repl").build();

while (scanner.hasNextLine()) {
    String input = scanner.nextLine();
    System.out.print("Claude: ");
    for (String text : client.llm("claude").stream(input, opts)) {
        System.out.print(text);
        System.out.flush();
    }
    System.out.println();
}
```

### Streaming with parameters

```java
ChatOptions opts = ChatOptions.builder()
        .model("gpt-4o")
        .temperature(0.5)
        .maxTokens(200)
        .system("Be concise.")
        .build();

for (String text : client.llm("gpt4o").stream("Explain async/await.", opts)) {
    System.out.print(text);
}
```

### Tool calls via `converse()`

```java
import io.github.sonicboom15.daimon.*;
import com.google.gson.JsonObject;

Tool getTempTool = new Tool(
    "get_temperature",
    "Get the current temperature for a city.",
    /* inputSchema JSON */ new JsonObject()
);

ChatOptions opts = ChatOptions.builder()
        .tools(List.of(getTempTool))
        .build();

List<Message> messages = List.of(Message.user("What's the weather in Tokyo?"));

for (Chunk chunk : client.llm("claude").converse(messages, opts)) {
    if (chunk.isText())     System.out.print(chunk.text());
    if (chunk.isToolCall()) System.out.println("[tool] " + chunk.toolCall().name());
}
```
