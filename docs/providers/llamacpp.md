# Local models (llamacpp)

The `llamacpp` component connects to any server that exposes an **OpenAI-compatible HTTP API**. This includes:

- [Ollama](https://ollama.com)
- [LM Studio](https://lmstudio.ai)
- [llama.cpp](https://github.com/ggerganov/llama.cpp) (server mode)

## Configuration

```yaml
components:
  - name: local
    type: llamacpp
    metadata:
      base_url: http://localhost:11434/v1   # required
      default_model: llama3.2:3b            # required
      api_key: local                         # optional; most local servers ignore it
    defaults:
      temperature: 0.7
      max_tokens: 2048
      system: "You are a helpful assistant."
```

## Metadata reference

| Key | Required | Description |
|---|---|---|
| `base_url` | **yes** | Full base URL including the `/v1` path segment |
| `default_model` | **yes** | Model name as recognised by your server |
| `api_key` | no | Bearer token. Defaults to `"local"` (most local servers ignore it). |

## Default base URLs

| Server | Default base URL |
|---|---|
| Ollama | `http://localhost:11434/v1` |
| LM Studio | `http://localhost:1234/v1` |
| llama.cpp | `http://localhost:8080/v1` |

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
| `tools` | ✓ (model-dependent) |

!!! note "Tool call support"
    Tool calls require a model that was fine-tuned for function calling. Small models (< 1B parameters) typically do not support them reliably. Models like `llama3.2:3b`, `qwen2.5:7b`, or `mistral:7b` work well.

## Setup guides

=== "Ollama"

    1. [Install Ollama](https://ollama.com/download) for your OS.

    2. Pull a model:
       ```bash
       ollama pull llama3.2:3b
       ```

    3. Ollama starts automatically (or run `ollama serve`). Configure daimon:
       ```yaml
       - name: local
         type: llamacpp
         metadata:
           base_url: http://localhost:11434/v1
           default_model: llama3.2:3b
       ```

=== "LM Studio"

    1. Download and install [LM Studio](https://lmstudio.ai).
    2. Download a model from the **Discover** tab.
    3. Go to **Local Server**, select your model, and click **Start Server**.
    4. Configure daimon:
       ```yaml
       - name: local
         type: llamacpp
         metadata:
           base_url: http://localhost:1234/v1
           default_model: lmstudio-community/Meta-Llama-3.1-8B-Instruct-GGUF
       ```

=== "llama.cpp"

    1. Build the [llama.cpp](https://github.com/ggerganov/llama.cpp) server.
    2. Download a GGUF model file.
    3. Start the server:
       ```bash
       ./llama-server -m your-model.gguf --port 8080 -c 4096
       ```
    4. Configure daimon:
       ```yaml
       - name: local
         type: llamacpp
         metadata:
           base_url: http://localhost:8080/v1
           default_model: your-model
       ```
