---
hide:
  - toc
---

# Stores

Daimon supports two categories of persistent stores that integrate directly into the component pipeline.

---

## Vector stores

Vector (document) stores enable semantic search over text content. Each store is exposed at `POST /v1/memory/{name}/query` and can be attached to any LLM component for automatic RAG enrichment.

| Type | External service | Embedding | Best for |
|---|---|---|---|
| [`inmemory`](vector/inmemory.md) | None | BM25 (lexical) | Development, testing, small corpora |
| [`chroma`](vector/chroma.md) | Chroma | Server-side | Quick setup, no embedding config |
| [`qdrant`](vector/qdrant.md) | Qdrant | Configurable endpoint | Production semantic search |
| [`redis`](vector/redis.md) | Redis Stack | Configurable endpoint | Existing Redis infrastructure |
| [`pgvector`](vector/pgvector.md) | PostgreSQL + pgvector | Configurable endpoint | Existing Postgres infrastructure |

### Two access modes

| Mode | Config | Best for |
|---|---|---|
| **Transparent RAG** | `memory_store: <name>` on an LLM component | Always-on background context |
| **Explicit tool calls** | Auto-generated `{name}_search` / `{name}_upsert` tools | Agentic reasoning, selective retrieval |

Both modes can be active simultaneously on the same LLM component.

---

## Graph stores

Graph stores model entities and relationships. Each store is exposed at `POST /v1/graph/{name}/cypher` and also generates tool definitions (`{name}_cypher`, `{name}_add_node`, `{name}_add_edge`) the LLM can call.

| Type | External service | Protocol | Best for |
|---|---|---|---|
| [`neo4j`](graph/neo4j.md) | Neo4j | Bolt (default) / HTTP | Knowledge graphs, production use |
| [`memgraph`](graph/memgraph.md) | Memgraph | Bolt (default) / HTTP | Fast in-memory graph analytics |

---

## Common HTTP API

All stores share the same URL shape. See the [HTTP API reference](../api.md) for full details.

**Vector stores:**

```
PUT    /v1/memory/{store}/{id}      Upsert with caller ID
POST   /v1/memory/{store}           Upsert, server assigns ID
POST   /v1/memory/{store}/query     Semantic search
DELETE /v1/memory/{store}/{id}      Delete document
```

**Graph stores:**

```
PUT    /v1/graph/{store}/nodes/{id}   Add/update node
POST   /v1/graph/{store}/nodes        Add node, server assigns ID
POST   /v1/graph/{store}/edges        Add directed edge
POST   /v1/graph/{store}/cypher       Run Cypher query
DELETE /v1/graph/{store}/nodes/{id}   Delete node + relationships
```
