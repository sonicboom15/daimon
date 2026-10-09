import type { ChoiceResult, Entity, VerifyResult } from './types.js';

export interface ExtractOptions {
  labels?: string[];
  threshold?: number;
}

export interface ChoiceOptions {
  state?: string;
}

export interface VerifyOptions {
  state?: string;
}

// ── NERClient ─────────────────────────────────────────────────────────────────

export class NERClient {
  private readonly base: string;
  private readonly model: string;
  private readonly timeout: number;

  constructor(base: string, model: string, timeout: number) {
    this.base = base;
    this.model = model;
    this.timeout = timeout;
  }

  /**
   * Extract named entities and spans from unstructured text.
   */
  async extract(text: string, options: ExtractOptions = {}): Promise<Entity[]> {
    const body: Record<string, unknown> = { text };
    if (options.labels !== undefined) body.labels = options.labels;
    if (options.threshold !== undefined) body.threshold = options.threshold;

    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), this.timeout);

    try {
      const resp = await fetch(`${this.base}/v1/ner/${this.model}/extract`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
        signal: controller.signal,
      });
      if (!resp.ok) throw new Error(`HTTP ${resp.status}: ${await resp.text()}`);
      const data = (await resp.json()) as { entities?: Entity[] };
      return (data.entities ?? []).map((e) => ({
        text: String(e.text ?? ''),
        label: String(e.label ?? ''),
        start: Number(e.start ?? 0),
        end: Number(e.end ?? 0),
        confidence: Number(e.confidence ?? 0),
      }));
    } finally {
      clearTimeout(timeoutId);
    }
  }
}

// ── DecisionClient ────────────────────────────────────────────────────────────

export class DecisionClient {
  private readonly base: string;
  private readonly model: string;
  private readonly timeout: number;

  constructor(base: string, model: string, timeout: number) {
    this.base = base;
    this.model = model;
    this.timeout = timeout;
  }

  /**
   * Select the single best match among candidate strings.
   */
  async choose(
    question: string,
    choices: string[],
    options: ChoiceOptions = {},
  ): Promise<ChoiceResult> {
    const body: Record<string, unknown> = {
      question,
      choices,
    };
    if (options.state) body.state = options.state;

    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), this.timeout);

    try {
      const resp = await fetch(`${this.base}/v1/decision/${this.model}/choose`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
        signal: controller.signal,
      });
      if (!resp.ok) throw new Error(`HTTP ${resp.status}: ${await resp.text()}`);
      const data = (await resp.json()) as Record<string, unknown>;
      const rawProbs = (data.probabilities ?? {}) as Record<string, unknown>;
      const probabilities: Record<string, number> = {};
      for (const [k, v] of Object.entries(rawProbs)) {
        probabilities[k] = Number(v);
      }
      return {
        selected: String(data.selected ?? ''),
        index: Number(data.index ?? 0),
        probabilities,
      };
    } finally {
      clearTimeout(timeoutId);
    }
  }

  /**
   * Evaluate calibrated truth probability for a boolean assertion.
   */
  async verify(statement: string, options: VerifyOptions = {}): Promise<VerifyResult> {
    const body: Record<string, unknown> = { statement };
    if (options.state) body.state = options.state;

    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), this.timeout);

    try {
      const resp = await fetch(`${this.base}/v1/decision/${this.model}/verify`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
        signal: controller.signal,
      });
      if (!resp.ok) throw new Error(`HTTP ${resp.status}: ${await resp.text()}`);
      const data = (await resp.json()) as Record<string, unknown>;
      return {
        probability: Number(data.probability ?? 0),
        supported: Boolean(data.supported ?? false),
      };
    } finally {
      clearTimeout(timeoutId);
    }
  }
}

