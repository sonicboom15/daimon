# Components

Each entry under `components:` defines one named provider instance. The `name` is used in request URLs: `POST /v1/converse/{name}`.

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
| `top_k` | int | Anthropic | Top-K sampling |
| `stop` | list of strings | all | Stop sequences |
| `frequency_penalty` | float | OpenAI, llamacpp | Frequency penalty |
| `presence_penalty` | float | OpenAI, llamacpp | Presence penalty |
| `seed` | int | OpenAI, llamacpp | RNG seed for reproducibility |
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
| `llamacpp` | — | Any OpenAI-compatible local server (Ollama, LM Studio, llama.cpp) |

See the [Providers](../providers/openai.md) section for per-provider metadata fields and supported parameters.
