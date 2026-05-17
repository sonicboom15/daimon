# Gemini

The `gemini` component connects to the **native Google Gemini REST API** using streaming SSE.

## Configuration

```yaml
components:
  - name: gemini
    type: gemini
    metadata:
      default_model: gemini-2.0-flash
      api_key: AIza...     # or set GEMINI_API_KEY / GOOGLE_API_KEY env var
    defaults:
      temperature: 1.0
      max_tokens: 2048
      top_p: 0.9
      top_k: 40
      system: "You are a helpful assistant."
```

## Metadata reference

| Key | Required | Description |
|---|---|---|
| `default_model` | no | Model used when the request doesn't specify one. Defaults to `gemini-2.0-flash`. |
| `api_key` | no | Falls back to `GEMINI_API_KEY`, then `GOOGLE_API_KEY` environment variable. |
| `base_url` | no | API base URL. Defaults to `https://generativelanguage.googleapis.com`. Override for Vertex AI or proxies. |

## Supported parameters

| Parameter | Supported | Notes |
|---|---|---|
| `temperature` | ✓ | |
| `max_tokens` | ✓ | Maps to `maxOutputTokens` |
| `top_p` | ✓ | |
| `top_k` | ✓ | **Gemini-specific** |
| `stop` | ✓ | Maps to `stopSequences` |
| `frequency_penalty` | — | Not supported; ignored |
| `presence_penalty` | — | Not supported; ignored |
| `seed` | — | Not supported; ignored |
| `system` | ✓ | Passed as `systemInstruction` |
| `tools` | ✓ | Mapped to `functionDeclarations` |

## Models

Any model available through the Gemini API can be specified in `default_model` or overridden per-request.

| Model | Notes |
|---|---|
| `gemini-2.0-flash` | Fast, efficient — recommended default |
| `gemini-2.0-flash-lite` | Lightest and cheapest |
| `gemini-2.5-pro` | Most capable |
| `gemini-2.5-flash` | Balanced performance |

```python
reply = client.llm("gemini").chat("Hello", model="gemini-2.5-pro")
```

## API key setup

Obtain a key from [Google AI Studio](https://aistudio.google.com/app/apikey):

```bash
export GEMINI_API_KEY=AIza...
```

The provider checks `api_key` metadata first, then `GEMINI_API_KEY`, then `GOOGLE_API_KEY`.

## Multiple instances

```yaml
components:
  - name: gemini-flash
    type: gemini
    metadata:
      default_model: gemini-2.0-flash

  - name: gemini-pro
    type: gemini
    metadata:
      default_model: gemini-2.5-pro
    defaults:
      max_tokens: 8192
      temperature: 0.7
```
