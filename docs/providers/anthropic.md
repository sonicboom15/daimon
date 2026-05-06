# Anthropic

The `anthropic` component connects to the Anthropic Messages API.

## Configuration

```yaml
components:
  - name: claude
    type: anthropic
    metadata:
      default_model: claude-opus-4-7
      api_key: sk-ant-...   # or set ANTHROPIC_API_KEY env var
    models:                 # optional: per-model API key overrides
      claude-haiku-4-5-20251001:
        api_key: sk-ant-haiku-key
    defaults:
      temperature: 1.0
      max_tokens: 4096
      top_p: 0.9
      top_k: 50
      stop: ["Human:"]
      system: "You are a helpful assistant."
```

## Metadata reference

| Key | Required | Description |
|---|---|---|
| `default_model` | **yes** (or `model`) | Model used when the request doesn't specify one |
| `api_key` | no | Falls back to `ANTHROPIC_API_KEY` environment variable |

## Supported parameters

| Parameter | Supported | Notes |
|---|---|---|
| `temperature` | ✓ | |
| `max_tokens` | ✓ | Defaults to 4096 if not set |
| `top_p` | ✓ | |
| `top_k` | ✓ | **Anthropic-specific** — ignored by other providers |
| `stop` | ✓ | |
| `frequency_penalty` | — | Not supported; ignored |
| `presence_penalty` | — | Not supported; ignored |
| `seed` | — | Not supported; ignored |
| `system` | ✓ | |
| `tools` | ✓ | |

## Models

| Model | Notes |
|---|---|
| `claude-opus-4-7` | Most capable |
| `claude-sonnet-4-6` | Balanced performance |
| `claude-haiku-4-5-20251001` | Fastest and cheapest |

## System prompt handling

Anthropic supports multiple system messages. When daimon resolves the effective system prompt, it follows this precedence:

1. `system` field in the request (highest priority)
2. A `role: system` message in `messages[]`
3. `defaults.system` in the component config (lowest priority)

If both a `system` field and a system-role message are present, they are concatenated.
