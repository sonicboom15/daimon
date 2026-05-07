# Neo4j

Type: `neo4j`

Connects to [Neo4j](https://neo4j.com/) using the Bolt binary protocol (default) or the HTTP Transactional Cypher API. Uses the official `neo4j-go-driver/v5`.

---

## Docker quickstart

```bash
docker run -d -p 7474:7474 -p 7687:7687 \
  -e NEO4J_AUTH=neo4j/password \
  neo4j:5
```

Browser UI available at `http://localhost:7474`.

---

## Configuration

```yaml
- name: kg
  type: neo4j
  metadata:
    bolt_url: bolt://localhost:7687     # Bolt (default protocol)
    protocol: bolt                      # "bolt" (default) or "http"
    http_url: http://localhost:7474     # used only when protocol: "http"
    database: neo4j
    username: neo4j
    password: secret
```

### Metadata keys

| Key | Default | Description |
|---|---|---|
| `bolt_url` | `bolt://localhost:7687` | Bolt connection URL |
| `http_url` | `http://localhost:7474` | HTTP base URL (used when `protocol: "http"`) |
| `protocol` | `bolt` | Transport: `"bolt"` or `"http"` |
| `database` | `neo4j` | Database name |
| `username` | `neo4j` | Username |
| `password` | — | Password |

---

## Relationship type validation

The `type` field in `POST /v1/graph/{store}/edges` must match `^[A-Za-z_][A-Za-z0-9_]*$`. Requests with invalid relationship types are rejected with `400 Bad Request` before any Cypher is executed.

---

## Notes

- **Bolt** is recommended: lower latency, connection pooling, native result streaming.
- **HTTP** is a fallback for environments where Bolt (port 7687) is blocked (e.g. certain cloud proxies). All Cypher queries work identically on both transports.
- Neo4j 5+ is tested. Neo4j 4.x should work but is not officially supported.
