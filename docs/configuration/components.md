# Components

Each entry under `components:` defines one named instance of any component type: LLM provider, embedder, session store, vector store, or graph store. The `name` is used in request URLs (`POST /v1/converse/{name}`, `POST /v1/memory/{name}/query`, etc.).

!!! tip "Declaration order"
    Embedders must be declared before any vector store that references them via `embedder:` metadata. Vector stores must be declared before any LLM component that references them via `memory_store:`. The sidecar resolves names in top-to-bottom order.

```yaml
components:
  - name: claude          # (required) unique name; used in the URL path
    type: anthropic       # (required) provider type — see Providers
    metadata:             # provider-specific settings
      default_model: claude-opus-4-7
      api_key: sk-ant-... # falls back to ANTHROPIC_API_KEY env var
    models:               # (optional) per-model API key overrides
      claude-haiku-4-5-20251001:
        api_key: sk-ant-haiku-key
    defaults:             # (optional) inference parameter defaults
      temperature: 1.0
      max_tokens: 4096
```

You can define as many components as you like — one per provider, or multiple instances of the same provider with different models or defaults.

---

## Inference parameter defaults

All fields under `defaults:` are optional. Request values always take precedence. Zero / nil means "use the provider's own default".

| Field | Type | Providers | Description |
|---|---|---|---|
| `temperature` | float | all | Sampling temperature |
| `max_tokens` | int | all | Maximum output tokens |
| `top_p` | float | all | Nucleus sampling probability |
| `top_k` | int | Anthropic, Gemini | Top-K sampling |
| `stop` | list of strings | all | Stop sequences |
| `frequency_penalty` | float | OpenAI, llamacpp, Mistral | Frequency penalty |
| `presence_penalty` | float | OpenAI, llamacpp, Mistral | Presence penalty |
| `seed` | int | OpenAI, llamacpp, Mistral | RNG seed for reproducibility |
| `system` | string | all | Default system prompt (used only when the request contains no system message) |

```yaml
defaults:
  temperature: 0.7
  max_tokens: 2048
  top_p: 0.9
  stop: ["Human:", "Assistant:"]
  system: "You are a helpful assistant."
```

---

## Provider types

| `type` | Env var | Notes |
|---|---|---|
| `anthropic` | `ANTHROPIC_API_KEY` | Claude models |
| `openai` | `OPENAI_API_KEY` | GPT models |
| `gemini` | `GEMINI_API_KEY` (or `GOOGLE_API_KEY`) | Google Gemini models via native REST API |
| `mistral` | `MISTRAL_API_KEY` | Mistral AI models |
| `llamacpp` | — | Any OpenAI-compatible local server (Ollama, LM Studio, llama.cpp) |

See the [Providers](../providers/openai.md) section for per-provider metadata fields and supported parameters.

---

## `memory_store` field (LLM components)

Add `memory_store: <store-name>` to any LLM component to enable **transparent RAG enrichment**. Before every `Chat` call, the sidecar queries the named vector store with the last user message and prepends the top results as a system message.

```yaml
- name: claude
  type: anthropic
  memory_store: chroma-docs     # must be declared above this entry
  metadata:
    default_model: claude-opus-4-7
```

The named store must be declared earlier in the `components:` list. If the query fails it is logged as a warning and the request continues normally.

---

## Embedder components

Embedders generate dense vector representations for text. Declare them before any vector store that needs them.

```yaml
- name: embedder
  type: embedding/openai
  metadata:
    base_url: https://api.openai.com   # or http://localhost:11434/v1 for Ollama
    api_key: sk-...                    # or OPENAI_API_KEY env var
    model: text-embedding-3-small
    dimensions: "1536"                 # optional override
```

The `embedding/openai` type works with any OpenAI-compatible embeddings endpoint (OpenAI, Ollama, LM Studio, etc.). Reference the embedder by name in a vector store's metadata:

```yaml
- name: qdrant-docs
  type: qdrant
  metadata:
    base_url: http://localhost:6333
    collection: daimon
    embedder: embedder        # must match the name above
```

---

## Session store components

By default sessions are stored in-memory and lost on restart. Declare a session store component to persist them. At most one session store may be configured.

```yaml
- name: sessions
  type: session/redis
  metadata:
    addr: localhost:6379
    password: ""
    db: "0"
    ttl: "24h"            # 0 or omit for no expiry
```

```yaml
- name: sessions
  type: session/postgres
  metadata:
    dsn: postgres://user:pass@localhost:5432/mydb
    table: daimon_sessions   # auto-created if absent
    ttl: "24h"
```

---

## Vector store components

See the [Stores](../stores/index.md) section for the full list of vector store types and their metadata keys.

## Graph store components

See the [Stores](../stores/index.md) section for the full list of graph store types and their metadata keys.
