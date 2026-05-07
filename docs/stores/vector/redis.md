# Redis Stack

Type: `redis`

Uses [Redis Stack](https://redis.io/docs/stack/) (the `redis/redis-stack` Docker image) with the RediSearch module for vector similarity search via `FT.CREATE` / `FT.SEARCH`.

!!! warning "Redis Stack required"
    Standard Redis does **not** include the FT commands. Use the `redis/redis-stack` or `redis/redis-stack-server` image, or enable the `redisearch` module manually.

---

## Docker quickstart

```bash
docker run -d -p 6379:6379 redis/redis-stack-server
```

---

## Configuration

```yaml
- name: embedder
  type: embedding/openai
  metadata:
    base_url: http://localhost:11434/v1
    model: nomic-embed-text
    dimensions: "768"

- name: redis-docs
  type: redis
  metadata:
    addr: localhost:6379
    index: daimon-docs
    embedder: embedder
    dimensions: "768"
```

### Metadata keys

| Key | Default | Description |
|---|---|---|
| `addr` | `localhost:6379` | Redis address |
| `password` | — | Redis password |
| `db` | `"0"` | Redis database number |
| `index` | `daimon` | RediSearch index name |
| `embedder` | — | Name of a declared `embedding/openai` component |
| `dimensions` | `"1536"` | Vector dimensions (must match the embedder) |

---

## Notes

- The FT index is created automatically on startup if it does not exist (HNSW, FLOAT32, cosine distance).
- Each document is stored as a Redis Hash keyed `{index}:{id}`.
- `dimensions` must match the `embedder`'s output size exactly. Mismatches produce a Redis error at query time.
