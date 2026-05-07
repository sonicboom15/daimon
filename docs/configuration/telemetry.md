# Telemetry

Daimon emits OpenTelemetry traces when an OTLP endpoint is configured. When the endpoint is empty (the default), tracing is a complete no-op — zero overhead.

```yaml
telemetry:
  otlp_endpoint: "localhost:4318"   # OTLP/HTTP. Leave empty to disable.
```

Each request to `/v1/converse/{component}` creates a root span. Providers create child spans for their upstream API calls.

---

See [Observability](../observability.md) for span attributes, a local Jaeger setup guide, and Grafana/Tempo configuration.
