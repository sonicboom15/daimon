# Chroma

Type: `chroma`

Connects to a [Chroma](https://www.trychroma.com/) server via its HTTP API. Chroma handles embeddings server-side using its default SentenceTransformer model — no `embedder:` component is needed.

**Best for:** quick semantic-search setup without configuring an external embedding endpoint.

---

## Docker quickstart

```bash
docker run -d -p 8000:8000 chromadb/chroma
```

---

## Configuration

```yaml
- name: chroma-docs
  type: chroma
  metadata:
    base_url: http://localhost:8000      # default
    collection: daimon                  # default
    create_if_missing: "true"           # auto-create collection on startup
```

### Metadata keys

| Key | Default | Description |
|---|---|---|
| `base_url` | `http://localhost:8000` | Chroma server URL |
| `collection` | `daimon` | Collection name |
| `create_if_missing` | `"false"` | Auto-create the collection if it doesn't exist |

---

## Notes

- Scores are computed as `1.0 - L2_distance` — higher is more similar.
- Chroma's default embedding model is `all-MiniLM-L6-v2`. Set the model via the Chroma server config, not via daimon.
- The `embedder:` metadata key is ignored — Chroma always embeds server-side.
