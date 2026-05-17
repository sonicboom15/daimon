---
hide:
  - navigation
---

# Tool Calls (MCP)

Daimon acts as an **MCP client**. It connects to [Model Context Protocol](https://modelcontextprotocol.io) servers at startup and automatically gives the model access to their tools — no code changes in your application needed.

---

## How it works

```
client  ──request──▶  daimon  ──Chat()──▶  LLM
                        │                   │
                        │     ◀── tool_call ┘
                        │
                        ├──CallTool()──▶  MCP server A (filesystem)
                        ├──CallTool()──▶  MCP server B (GitHub)
                        │
                        └──Chat() with tool results──▶  LLM
                                                         │
                                            ◀── text ────┘
```

1. At startup, daimon connects to each configured MCP server and fetches its tool catalogue.
2. Every chat request has all MCP tools injected automatically.
3. When the model calls a tool, daimon executes it via the owning MCP server.
4. The result is appended to the conversation and the model is called again.
5. This loop repeats until the model returns a plain text response.

Your application receives a single streaming response. `tool_call` events are forwarded as SSE chunks so your UI can show progress.

---

## Configuration

```yaml
mcp_servers:
  - name: filesystem          # logical name — used in logs
    command:                  # argv to start the server process
      - npx
      - -y
      - "@modelcontextprotocol/server-filesystem"
      - /tmp

  - name: github
    command:
      - npx
      - -y
      - "@modelcontextprotocol/server-github"
```

Daimon spawns each server as a subprocess and communicates via **stdio JSON-RPC 2.0**. The `command` array is the full argv — no shell interpolation.

!!! note
    MCP server startup failures are non-fatal. Daimon logs a warning and continues without that server's tools. Check logs if tools are not showing up.

---

## Available MCP servers

The MCP ecosystem has a growing list of ready-made servers. Some popular ones:

| Server | npm package | What it provides |
|---|---|---|
| Filesystem | `@modelcontextprotocol/server-filesystem` | Read/write files in a sandboxed directory |
| GitHub | `@modelcontextprotocol/server-github` | Search repos, issues, PRs |
| Brave Search | `@modelcontextprotocol/server-brave-search` | Web search |
| Postgres | `@modelcontextprotocol/server-postgres` | Query a Postgres database |
| SQLite | `@modelcontextprotocol/server-sqlite` | Query a SQLite database |
| Fetch | `@modelcontextprotocol/server-fetch` | HTTP GET arbitrary URLs |

Browse the full list at [modelcontextprotocol.io/servers](https://modelcontextprotocol.io/servers).

---

## Example: filesystem tools

Give the model access to `/tmp`:

```yaml
mcp_servers:
  - name: filesystem
    command:
      - npx
      - -y
      - "@modelcontextprotocol/server-filesystem"
      - /tmp
```

Then ask it to create a file:

```bash
curl -sN http://127.0.0.1:3500/v1/converse/claude \
  -H "Content-Type: application/json" \
  -d '{"messages":[{"role":"user","content":"Create a file /tmp/hello.txt with the text Hello world"}]}'
```

```
data: {"type":"tool_call","tool_call":{"id":"call_1","name":"write_file","input":{"path":"/tmp/hello.txt","content":"Hello world"}}}
data: {"type":"text","text":"I've created the file /tmp/hello.txt with the text \"Hello world\"."}
data: {"type":"done"}
```

---

## Observing tool calls in Python

The `on_tool_call` callback fires whenever the model calls a tool:

```python
import daimon_client as daimon

def on_tool(tc: daimon.ToolCall) -> None:
    print(f"\n→ {tc.name}({tc.input})", flush=True)

with daimon.Client() as client:
    for text in client.llm("claude").stream(
        "List the files in /tmp and summarise what you find.",
        on_tool_call=on_tool,
    ):
        print(text, end="", flush=True)
print()
```

Output:
```
→ list_directory({"path": "/tmp"})
The /tmp directory contains 3 files: hello.txt, tmp123, and data.json.
```

---

## Providing your own tools

You can pass tools directly in the request alongside (or instead of) MCP tools:

```python
search_tool = daimon.Tool(
    name="search",
    description="Search internal knowledge base.",
    input_schema={
        "type": "object",
        "properties": {"query": {"type": "string"}},
        "required": ["query"],
    },
)

for chunk in client.llm("claude").converse(messages=messages, tools=[search_tool]):
    if chunk.type == "tool_call":
        # Execute the tool yourself and append the result to messages
        result = my_search(chunk.tool_call.input["query"])
        # ...
```

!!! tip
    When you provide tools via the request, you are responsible for executing them and appending results to the conversation. Only MCP tools configured in YAML are executed automatically by daimon.

---

## Agentic loop details

The loop runs entirely inside daimon:

- **No maximum iterations** — the loop continues until the model returns a response with no tool calls. Design your tools and prompts accordingly.
- **Tool errors** — if a tool call fails, the error message is returned as the tool result so the model can react gracefully.
- **Context growth** — each tool call round appends one assistant turn and one or more tool result turns. Very long chains can approach model context limits.
- **Cancellation** — if the client disconnects, the request context is cancelled and the loop stops cleanly.
