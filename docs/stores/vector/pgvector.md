# pgvector

Type: `pgvector`

Uses PostgreSQL with the [`pgvector`](https://github.com/pgvector/pgvector) extension for cosine similarity search via the `<=>` operator.

---

## Docker quickstart

```bash
docker run -d -p 5432:5432 \
  -e POSTGRES_PASSWORD=secret \
  pgvector/pgvector:pg16
```

The `pgvector/pgvector` image includes the extension pre-installed. For standard Postgres, install the extension manually:

```sql
CREATE EXTENSION IF NOT EXISTS vector;
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

- name: pg-docs
  type: pgvector
  metadata:
    dsn: postgres://user:secret@localhost:5432/mydb
    table: daimon_documents
    embedder: embedder
    dimensions: "768"
```

### Metadata keys

| Key | Default | Description |
|---|---|---|
| `dsn` | — | **Required.** Postgres connection string |
| `table` | `daimon_documents` | Table name (auto-created if absent) |
| `embedder` | — | Name of a declared `embedding/openai` component |
| `dimensions` | `"1536"` | Vector dimensions (must match the embedder) |

---

## Auto-created table schema

```sql
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS daimon_documents (
    id        TEXT PRIMARY KEY,
    content   TEXT NOT NULL,
    metadata  JSONB,
    embedding vector(768)
);

CREATE INDEX IF NOT EXISTS daimon_documents_embedding_idx
    ON daimon_documents USING ivfflat (embedding vector_cosine_ops);
```

The table and index are created on first startup. If the table already exists with a different schema, startup will fail.

---

## Notes

- Scores are `1 - cosine_distance`, so 1.0 is an exact match.
- The IVFFlat index requires at least a few hundred rows before it improves query performance; for small datasets the sequential scan is used automatically.
- `dimensions` must match the `embedder`'s output exactly. Changing dimensions requires dropping and recreating the table.
