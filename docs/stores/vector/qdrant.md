# Qdrant

Type: `qdrant`

Connects to a [Qdrant](https://qdrant.tech/) vector database via its REST API. Daimon calls a configurable embeddings endpoint to generate vectors before upserting or querying.

**Best for:** production semantic search with a dedicated vector database.

---

## Docker quickstart

```bash
docker run -d -p 6333:6333 qdrant/qdrant
```

---

## Configuration

```yaml
# Declare the embedder first
- name: embedder
  type: embedding/openai
  metadata:
    base_url: http://localhost:11434/v1   # Ollama
    model: nomic-embed-text
    dimensions: "768"

- name: qdrant-docs
  type: qdrant
  metadata:
    base_url: http://localhost:6333
    collection: daimon
    embedder: embedder            # reference the component above
    create_if_missing: "true"
    # api_key: ...                # Qdrant Cloud only
```

### Metadata keys

| Key | Default | Description |
|---|---|---|
| `base_url` | `http://localhost:6333` | Qdrant server URL |
| `collection` | `daimon` | Collection name |
| `embedder` | — | Name of a declared `embedding/openai` component |
| `create_if_missing` | `"false"` | Auto-create collection on startup |
| `dimensions` | `"1536"` | Vector dimensions (must match the embedder) |
| `api_key` | — | Qdrant Cloud API key |

---

## Notes

- If `embedder` is not set, the store falls back to a deterministic hash vector — useful for configuration smoke tests but not semantically meaningful.
- Scores are Qdrant cosine similarity values (0–1, higher is more similar) when the collection is configured with cosine distance.
- The collection's distance metric is set at creation time. If the collection already exists, `create_if_missing` has no effect on its metric.
