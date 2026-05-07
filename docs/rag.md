---
hide:
  - navigation
---

# RAG Enrichment

Daimon supports **transparent Retrieval-Augmented Generation (RAG)**: before every chat request, the sidecar automatically queries a vector store and prepends the top results as a system message. The LLM receives relevant context without any change to your client code.

---

## How it works

1. A vector store is configured and loaded with documents.
2. An LLM component declares `memory_store: <store-name>`.
3. On each `/v1/converse/{component}` request, daimon:
   - Finds the last `user` message in the request.
   - Queries the named vector store for the top 5 most relevant documents.
   - Prepends a system message containing those documents to the conversation history.
   - Calls the LLM with the enriched history.

The client sees no difference in the API surface — it still sends messages and receives SSE chunks.

---

## Configuration

Declare the store before the LLM component, then add `memory_store:` to the LLM component:

```yaml
components:

  # 1. Embedder (if the store needs one)
  - name: embedder
    type: embedding/openai
    metadata:
      base_url: http://localhost:11434/v1
      model: nomic-embed-text
      dimensions: "768"

  # 2. Vector store
  - name: docs
    type: qdrant
    metadata:
      base_url: http://localhost:6333
      collection: product-docs
      embedder: embedder
      create_if_missing: "true"

  # 3. LLM with RAG enabled
  - name: claude
    type: anthropic
    memory_store: docs              # ← enables transparent RAG
    metadata:
      default_model: claude-opus-4-7
```

---

## Populating the store

Use the memory HTTP API or the SDK before the first chat request:

=== "Python"

    ```python
    import daimon

    client = daimon.Client()
    store = client.memory("docs")

    # Bulk-load documents
    for doc in my_documents:
        store.upsert(doc["text"], metadata={"source": doc["url"]})
    ```

=== "TypeScript"

    ```typescript
    import { Client } from 'daimon-client';

    const client = new Client();
    const store = client.memory('docs');

    for (const doc of myDocuments) {
        await store.upsert(doc.text, { metadata: { source: doc.url } });
    }
    ```

=== "curl"

    ```bash
    curl -X POST http://127.0.0.1:3500/v1/memory/docs \
      -H "Content-Type: application/json" \
      -d '{"content": "Daimon listens on port 3500 by default.", "metadata": {"source": "docs"}}'
    ```

---

## Explicit tool access

In addition to transparent RAG, every vector store automatically generates two tools the LLM can call:

- `{store_name}_search` — search for documents matching a query
- `{store_name}_upsert` — store a new document

These are injected alongside MCP tools and available in every chat request. The LLM can choose *when* to search rather than having context injected on every turn — useful for agentic workloads where retrieval is conditional.

```python
# No special client code needed — the LLM decides when to call the tools
reply = client.chat("claude", "What does our documentation say about authentication?")
# The model may call docs_search("authentication") automatically
```

---

## Choosing a store

| Store | Embedding | Good for |
|---|---|---|
| `inmemory` | BM25 (lexical) | Development, keyword-heavy docs |
| `chroma` | Server-side (SentenceTransformer) | Quick setup, no embedding config |
| `qdrant` | Configurable endpoint | Production, high-volume |
| `redis` | Configurable endpoint | Existing Redis Stack infrastructure |
| `pgvector` | Configurable endpoint | Existing Postgres infrastructure |

For production RAG, prefer `qdrant` or `pgvector` with `nomic-embed-text` (via Ollama) or `text-embedding-3-small` (OpenAI).

---

## Failure behaviour

If the vector store query fails (e.g. the backend is down), the error is logged as a warning and the request proceeds without enrichment. The LLM receives the original message history unchanged.
