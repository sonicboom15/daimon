# Mistral

The `mistral` component connects to the **Mistral AI API**. Mistral exposes an OpenAI-compatible endpoint, so this provider reuses the same streaming client as `openai` — no additional dependency.

## Configuration

```yaml
components:
  - name: mistral
    type: mistral
    metadata:
      default_model: mistral-large-latest
      api_key: ...     # or set MISTRAL_API_KEY env var
    defaults:
      temperature: 0.7
      max_tokens: 2048
      top_p: 0.9
      system: "You are a helpful assistant."
```

## Metadata reference

| Key | Required | Description |
|---|---|---|
| `default_model` | no | Model used when the request doesn't specify one. Defaults to `mistral-large-latest`. |
| `api_key` | no | Falls back to `MISTRAL_API_KEY` environment variable. |
| `base_url` | no | API base URL. Defaults to `https://api.mistral.ai/v1`. |

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

Any model available through the Mistral API can be specified in `default_model` or overridden per-request.

| Model | Notes |
|---|---|
| `mistral-large-latest` | Most capable |
| `mistral-small-latest` | Faster, lower cost |
| `open-mistral-nemo` | Open-weight, Apache 2.0 |
| `codestral-latest` | Code-focused |

```python
reply = client.llm("mistral").chat("Hello", model="codestral-latest")
```

## API key setup

Obtain a key from the [Mistral console](https://console.mistral.ai/):

```bash
export MISTRAL_API_KEY=...
```

## Multiple instances

```yaml
components:
  - name: mistral-large
    type: mistral
    metadata:
      default_model: mistral-large-latest

  - name: codestral
    type: mistral
    metadata:
      default_model: codestral-latest
    defaults:
      temperature: 0.2
      max_tokens: 4096
```
