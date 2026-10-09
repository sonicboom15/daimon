# Decision Models

The `decision` component family evaluates discrete questions (`choose`) and assertion verifications (`verify`) against context using non-autoregressive, calibrated discriminative models.

This allows Daimon pipelines to perform deterministic leaf-node disambiguation, clinical coding matching, and hallucination checks without token generation overhead.

---

## Configuration

Declare decision models under `components:` in your YAML config:

### 1. HTTP Webhook (`decision/http`)

Forwards requests to any external service implementing `/choose` and `/verify`:

```yaml
components:
  - name: verifier
    type: decision/http
    metadata:
      base_url: http://localhost:8000
      api_key: optional-bearer-token
      timeout_ms: "800"
```

### 2. TypeSafe / Jev (`decision/jev`)

Connects to the TypeSafe Jev calibrated discriminative decision engine:

```yaml
components:
  - name: verifier
    type: decision/jev
    metadata:
      api_key: ${JEV_API_KEY}
      base_url: https://api.typesafe.com/v1
      default_model: jev-v1
      timeout_ms: "800"
```

---

## Auto-Generated LLM Tools

Declaring a decision model registers two tools for LLM agent use:

### 1. `{name}_choose`
* **Description:** Selects the single best match among a list of candidate strings given context.
* **Parameters:**
  * `question` (string, required): The target question.
  * `choices` (array of strings, required): Candidate list (up to 255 items).
  * `state` (string, optional): Context excerpt. Defaults to active conversation history if omitted.

### 2. `{name}_verify`
* **Description:** Returns calibrated truth probability for a boolean assertion.
* **Parameters:**
  * `statement` (string, required): Statement to verify against state.
  * `state` (string, optional): Context excerpt.

---

## HTTP REST API

### `POST /v1/decision/{name}/choose`

```bash
curl -s http://127.0.0.1:3500/v1/decision/verifier/choose \
  -H "Content-Type: application/json" \
  -d '{
    "state": "Patient presents with right knee pain following a sports injury.",
    "question": "Which ICD-10 code matches laterality?",
    "choices": ["M25.561", "M25.562", "M25.569"]
  }'
```

**Response:**

```json
{
  "selected": "M25.561",
  "index": 0,
  "probabilities": {
    "M25.561": 0.97,
    "M25.562": 0.02,
    "M25.569": 0.01
  }
}
```

### `POST /v1/decision/{name}/verify`

```bash
curl -s http://127.0.0.1:3500/v1/decision/verifier/verify \
  -H "Content-Type: application/json" \
  -d '{
    "state": "Patient presents with right knee pain. Examination reveals joint effusion.",
    "statement": "Right knee joint effusion is documented."
  }'
```

**Response:**

```json
{
  "probability": 0.98,
  "supported": true
}
```

---

## SDK Usage

=== "Python"

    ```python
    import daimon

    client = daimon.Client()
    verifier = client.decision("verifier")

    # 1. Choose best matching code
    choice = verifier.choose(
        question="Which ICD-10 code matches laterality?",
        choices=["M25.561", "M25.562", "M25.569"],
        state="Patient presents with right knee pain following a sports injury.",
    )
    print(f"Selected: {choice.selected} (p={choice.probabilities[choice.selected]:.2f})")

    # 2. Verify boolean assertion
    verify = verifier.verify(
        statement="Right knee joint effusion is documented.",
        state="Patient presents with right knee pain. Examination reveals joint effusion.",
    )
    print(f"Supported: {verify.supported} (p={verify.probability:.2f})")
    ```

=== "TypeScript"

    ```typescript
    import { Client } from 'daimon-client';

    const client = new Client();
    const verifier = client.decision('verifier');

    // 1. Choose best matching code
    const choice = await verifier.choose(
      'Which ICD-10 code matches laterality?',
      ['M25.561', 'M25.562', 'M25.569'],
      { state: 'Patient presents with right knee pain following a sports injury.' }
    );
    console.log(`Selected: ${choice.selected}`);

    // 2. Verify boolean assertion
    const verify = await verifier.verify('Right knee joint effusion is documented.', {
      state: 'Patient presents with right knee pain. Examination reveals joint effusion.',
    });
    console.log(`Supported: ${verify.supported} (p=${verify.probability})`);
    ```

=== "Java"

    ```java
    import io.github.sonicboom15.daimon.ChoiceResult;
    import io.github.sonicboom15.daimon.Client;
    import io.github.sonicboom15.daimon.DecisionClient;
    import io.github.sonicboom15.daimon.VerifyResult;
    import java.util.List;

    Client client = new Client();
    DecisionClient verifier = client.decision("verifier");

    ChoiceResult choice = verifier.choose(
        "Which ICD-10 code matches laterality?",
        List.of("M25.561", "M25.562", "M25.569"),
        "Patient presents with right knee pain following a sports injury."
    );

    VerifyResult verify = verifier.verify(
        "Right knee joint effusion is documented.",
        "Patient presents with right knee pain. Examination reveals joint effusion."
    );
    ```

