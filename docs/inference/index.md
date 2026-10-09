---
hide:
  - toc
---

# Inference Primitives

In addition to generative LLMs, Daimon manages two high-throughput deterministic and calibrated inference primitives: **NER** and **Decision Models**.

These components allow Daimon to support hybrid domain pipelines (such as medical coding and automated entity disambiguation) without burning expensive generative LLM tokens on span extraction or leaf-node classification.

---

## Component Categories

| Category | Type Prefix | Description | Primary Drivers |
|---|---|---|---|
| **[NER](ner.md)** | `ner/*` | High-throughput entity extraction, span detection, character offsets, and confidence scoring | `ner/http`, `ner/onnx` |
| **[Decision](decision.md)** | `decision/*` | Non-autoregressive calibrated discriminative models for candidate ranking and statement verification | `decision/http`, `decision/jev` |

---

## End-to-End Pipeline Architecture

By combining NER, Vector Stores, Knowledge Graphs, and Decision models, Daimon executes deterministic extraction and disambiguation pipelines:

```
Raw Unstructured Text
       │
       ▼
[POST /v1/ner/clinical-ner/extract]
       │
       ├─► Returns Entities: ["right knee pain", "mild asthma"]
       │
       ▼
[POST /v1/memory/icd10/query] (Vector Search)
       │
       ├─► Returns Category Nodes: [M25.5, J45.20]
       │
       ▼
[POST /v1/graph/icd10-graph/cypher] (Knowledge Graph)
       │
       ├─► Expands Sub-codes for M25.5: [M25.561, M25.562, M25.569]
       │
       ▼
[POST /v1/decision/verifier/choose]
       │
       ├─ State: "Patient presents with right knee pain..."
       ├─ Question: "Which code accurately describes laterality?"
       ├─ Choices: ["M25.561", "M25.562", "M25.569"]
       │
       ▼
Final Output: "M25.561" (Confidence: 0.97)
```

---

## Agentic Loop & Tool Synthesis

When an NER or Decision component is declared in `config.yaml`, Daimon registers corresponding tools in the sidecar's tool catalogue. Any configured LLM can call these tools directly during conversational loops:

* `{name}_extract`: Extract spans and entities from text.
* `{name}_choose`: Select the highest-probability choice from candidate options.
* `{name}_verify`: Evaluate calibrated truth probability for a boolean assertion.

