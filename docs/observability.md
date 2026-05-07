---
hide:
  - navigation
---

# Observability

Daimon emits **OpenTelemetry traces** for every request. When an OTLP endpoint is configured, each call to `/v1/converse/{component}` creates a root span with the provider and model as attributes.

---

## Configuration

```yaml
telemetry:
  otlp_endpoint: "localhost:4318"   # OTLP/HTTP. Leave empty to disable.
```

When the endpoint is empty (the default), all tracing is a no-op — zero overhead.

---

## Span structure

Each request produces a single root span named `converse`:

| Attribute | Value |
|---|---|
| `gen_ai.component` | Component name (e.g. `claude`) |
| `gen_ai.request.model` | Model name from the request |

Providers should create child spans for their upstream API calls and annotate them with [OpenTelemetry GenAI semantic conventions](https://opentelemetry.io/docs/specs/semconv/gen-ai/):

| Attribute | Example |
|---|---|
| `gen_ai.system` | `anthropic` |
| `gen_ai.request.model` | `claude-opus-4-7` |
| `gen_ai.usage.input_tokens` | `42` |
| `gen_ai.usage.output_tokens` | `128` |

---

## Local setup with Jaeger

The quickest way to see traces locally:

```bash
docker run -d --name jaeger \
  -p 4318:4318 \
  -p 16686:16686 \
  jaegertracing/jaeger:latest
```

Configure daimon:

```yaml
telemetry:
  otlp_endpoint: "localhost:4318"
```

Open the Jaeger UI at [http://localhost:16686](http://localhost:16686), select **daimon** in the service dropdown, and click **Find Traces**.

---

## Grafana / production

Any OTLP-compatible collector works. For Grafana Tempo:

```yaml
telemetry:
  otlp_endpoint: "tempo.internal:4318"
```

Or route through the OpenTelemetry Collector:

```yaml
telemetry:
  otlp_endpoint: "otel-collector:4318"
```
