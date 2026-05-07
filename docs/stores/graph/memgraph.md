# Memgraph

Type: `memgraph`

Connects to [Memgraph](https://memgraph.com/) using the Bolt binary protocol (default) or Memgraph's HTTP query endpoint. Uses the same `neo4j-go-driver/v5` as the Neo4j component — the driver is Memgraph-compatible.

---

## Docker quickstart

```bash
docker run -d -p 7687:7687 -p 7444:7444 memgraph/memgraph
```

---

## Configuration

```yaml
- name: mg
  type: memgraph
  metadata:
    bolt_url: bolt://localhost:7687
    protocol: bolt                   # "bolt" (default) or "http"
    http_url: http://localhost:7444  # used only when protocol: "http"
    # username and password are optional for local Memgraph
```

### Metadata keys

| Key | Default | Description |
|---|---|---|
| `bolt_url` | `bolt://localhost:7687` | Bolt connection URL |
| `http_url` | `http://localhost:7444` | HTTP base URL (used when `protocol: "http"`) |
| `protocol` | `bolt` | Transport: `"bolt"` or `"http"` |
| `username` | — | Optional username |
| `password` | — | Optional password |

---

## Notes

- Memgraph speaks openCypher — the same Cypher queries used for Neo4j work here.
- The HTTP transport sends `POST /query` with `{"query": "...", "parameters": {...}}` body (different from Neo4j's `/db/{db}/tx/commit` shape).
- Relationship type validation (`^[A-Za-z_][A-Za-z0-9_]*$`) is enforced identically to the Neo4j component.
