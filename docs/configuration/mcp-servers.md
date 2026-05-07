# MCP Servers

Daimon connects to each listed MCP server at startup (as a client, over stdio). Their tools are injected into every chat request automatically — no code changes needed in your application.

```yaml
mcp_servers:
  - name: filesystem    # logical name (used in logs)
    command:            # argv to start the server process
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

The `command` array is the full argv — no shell expansion or interpolation.

!!! note
    MCP server startup failures are non-fatal. Daimon logs a warning and continues without that server's tools.

---

## Popular servers

| Server | npm package | What it provides |
|---|---|---|
| Filesystem | `@modelcontextprotocol/server-filesystem` | Read/write files in a sandboxed directory |
| GitHub | `@modelcontextprotocol/server-github` | Search repos, issues, PRs |
| Brave Search | `@modelcontextprotocol/server-brave-search` | Web search |
| Postgres | `@modelcontextprotocol/server-postgres` | Query a Postgres database |
| SQLite | `@modelcontextprotocol/server-sqlite` | Query a SQLite database |
| Fetch | `@modelcontextprotocol/server-fetch` | HTTP GET arbitrary URLs |

Browse the full catalogue at [modelcontextprotocol.io/servers](https://modelcontextprotocol.io/servers).

---

See [Tool Calls (MCP)](../mcp.md) for the full guide including the agentic loop, observing tool calls in SDKs, and providing your own tools.
