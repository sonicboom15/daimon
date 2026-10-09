# Named Entity Recognition (NER)

The `ner` component family provides entity extraction, span detection, and character offset tracking.

Instead of prompting an LLM to output JSON spans, Daimon delegates span extraction to specialized models (such as GLiNER, BioLinkBERT, or token-classification networks), cutting latency and cost.

---

## Configuration

Declare NER components under `components:` in your YAML config:

### 1. HTTP Microservice (`ner/http`)

Connects to any external or containerized microservice exposing an `/extract` endpoint (e.g., FastAPI, Triton Inference Server, or Hugging Face TEI):

```yaml
components:
  - name: clinical-ner
    type: ner/http
    metadata:
      base_url: http://localhost:8000
      api_key: optional-bearer-token
      default_labels: "disease,symptom,procedure"
      default_threshold: "0.60"
      timeout_ms: "5000"
```

### 2. Local ONNX Runtime (`ner/onnx`)

Executes an exported ONNX model directly:

```yaml
components:
  - name: clinical-ner-onnx
    type: ner/onnx
    metadata:
      model_path: /models/gliner-biomed-small.onnx
      default_labels: "disease,symptom,procedure"
      default_threshold: "0.60"
      threads: "4"
```

---

## Auto-Generated LLM Tool

When configured, Daimon injects an auto-generated tool into every LLM request:

* **Tool Name:** `{name}_extract`
* **Parameters:**
  * `text` (string, required): Unstructured text to extract spans from.
  * `labels` (array of strings, optional): Custom labels overriding defaults.

When the LLM calls `{name}_extract`, the sidecar runs inference and returns the entity list back to the model within the agentic loop.

---

## HTTP REST API

Call the endpoint directly via HTTP:

```bash
curl -s http://127.0.0.1:3500/v1/ner/clinical-ner/extract \
  -H "Content-Type: application/json" \
  -d '{
    "text": "Patient has right knee effusion and mild asthma.",
    "labels": ["disease", "symptom", "procedure"],
    "threshold": 0.60
  }'
```

**Response:**

```json
{
  "entities": [
    {
      "text": "right knee effusion",
      "label": "symptom",
      "start": 12,
      "end": 31,
      "confidence": 0.96
    },
    {
      "text": "asthma",
      "label": "disease",
      "start": 46,
      "end": 52,
      "confidence": 0.94
    }
  ]
}
```

---

## SDK Usage

=== "Python"

    ```python
    import daimon

    client = daimon.Client()
    entities = client.ner("clinical-ner").extract(
        "Patient has right knee effusion and mild asthma.",
        labels=["disease", "symptom", "procedure"],
        threshold=0.60,
    )
    for entity in entities:
        print(f"{entity.text} ({entity.label}) [{entity.start}:{entity.end}] score={entity.confidence:.2f}")
    ```

=== "TypeScript"

    ```typescript
    import { Client } from 'daimon-client';

    const client = new Client();
    const entities = await client.ner('clinical-ner').extract(
      'Patient has right knee effusion and mild asthma.',
      {
        labels: ['disease', 'symptom', 'procedure'],
        threshold: 0.60,
      }
    );
    for (const entity of entities) {
      console.log(`${entity.text} (${entity.label}) score=${entity.confidence}`);
    }
    ```

=== "Java"

    ```java
    import io.github.sonicboom15.daimon.Client;
    import io.github.sonicboom15.daimon.Entity;
    import java.util.List;

    Client client = new Client();
    List<Entity> entities = client.ner("clinical-ner").extract(
        "Patient has right knee effusion and mild asthma.",
        List.of("disease", "symptom", "procedure"),
        0.60
    );
    ```

