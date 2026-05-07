# In-Memory Store

Type: `inmemory`

An in-process BM25 lexical store with no external dependencies. Scores documents using term frequency / inverse document frequency (BM25, k1=1.2, b=0.75). All data is lost when the sidecar restarts.

**Best for:** development, unit tests, small static corpora where exact-keyword retrieval is sufficient.

---

## Configuration

```yaml
- name: docs
  type: inmemory
```

No `metadata` keys are required. No external service needed.

---

## Notes

- Retrieval is **lexical**, not semantic — there is no embedding model. The `query` text is tokenised and BM25-scored against stored documents.
- For semantic similarity search, use `chroma`, `qdrant`, `redis`, or `pgvector` with a configured embedder.
- IDs are UUID v4 strings when the caller omits the `id` field.
