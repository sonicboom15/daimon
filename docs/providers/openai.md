# OpenAI

The `openai` component connects to the OpenAI chat completions API.

## Configuration

```yaml
components:
  - name: gpt4o           # name used in request URLs
    type: openai
    metadata:
      default_model: gpt-4o
      api_key: sk-...     # or set OPENAI_API_KEY env var
    models:               # optional: per-model API key overrides
      o1-preview:
        api_key: sk-o1-specific-key
    defaults:
      temperature: 0.7
      max_tokens: 2048
      top_p: 0.9
      stop: ["Human:"]
      frequency_penalty: 0.0
      presence_penalty: 0.0
      seed: 42
      system: "You are a helpful assistant."
```

## Metadata reference

| Key | Required | Description |
|---|---|---|
| `default_model` | **yes** (or `model`) | Model used when the request doesn't specify one |
| `api_key` | no | Falls back to `OPENAI_API_KEY` environment variable |

## Supported parameters

| Parameter | Supported |
|---|---|
| `temperature` | ✓ |
| `max_tokens` | ✓ |
| `top_p` | ✓ |
| `top_k` | — (ignored) |
| `stop` | ✓ |
| `frequency_penalty` | ✓ |
| `presence_penalty` | ✓ |
| `seed` | ✓ |
| `system` | ✓ |
| `tools` | ✓ |

## Models

Any model available through the OpenAI API can be specified in `default_model` or overridden per-request:

```python
reply = client.llm("gpt4o").chat("Hello", model="o1-preview")
```

Common choices:

| Model | Notes |
|---|---|
| `gpt-4o` | Best quality |
| `gpt-4o-mini` | Faster, cheaper |
| `o1-preview` | Reasoning model |
| `o1-mini` | Faster reasoning |

## Multiple instances

You can configure multiple OpenAI components (e.g. for different API keys or defaults):

```yaml
components:
  - name: gpt4o
    type: openai
    metadata:
      default_model: gpt-4o
  - name: gpt4o-mini
    type: openai
    metadata:
      default_model: gpt-4o-mini
    defaults:
      max_tokens: 512
      temperature: 0.3
```
