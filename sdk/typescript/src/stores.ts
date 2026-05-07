import type { MemoryResult } from './types.js';

export interface UpsertOptions {
  id?: string;
  metadata?: Record<string, string>;
}

export interface AddNodeOptions {
  id?: string;
  labels?: string[];
  props?: Record<string, unknown>;
}

export interface AddEdgeOptions {
  props?: Record<string, unknown>;
}

// ── MemoryStoreClient ─────────────────────────────────────────────────────────

export class MemoryStoreClient {
  private readonly base: string;
  private readonly store: string;
  private readonly timeout: number;

  constructor(base: string, store: string, timeout: number) {
    this.base = base;
    this.store = store;
    this.timeout = timeout;
  }

  async upsert(content: string, options: UpsertOptions = {}): Promise<string> {
    const body: Record<string, unknown> = { content };
    if (options.metadata) body.metadata = options.metadata;

    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), this.timeout);

    try {
      const url = options.id
        ? `${this.base}/v1/memory/${this.store}/${options.id}`
        : `${this.base}/v1/memory/${this.store}`;
      const method = options.id ? 'PUT' : 'POST';

      const resp = await fetch(url, {
        method,
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
        signal: controller.signal,
      });
      if (!resp.ok) throw new Error(`HTTP ${resp.status}: ${await resp.text()}`);
      const data = (await resp.json()) as Record<string, unknown>;
      return String(data.id ?? '');
    } finally {
      clearTimeout(timeoutId);
    }
  }

  async query(query: string, topK = 5): Promise<MemoryResult[]> {
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), this.timeout);

    try {
      const resp = await fetch(`${this.base}/v1/memory/${this.store}/query`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ query, top_k: topK }),
        signal: controller.signal,
      });
      if (!resp.ok) throw new Error(`HTTP ${resp.status}: ${await resp.text()}`);
      const data = (await resp.json()) as { results?: unknown[] };
      const raw = data.results ?? [];
      return raw.map((r) => {
        const row = r as Record<string, unknown>;
        return {
          id: String(row.id ?? ''),
          content: String(row.content ?? ''),
          metadata:
            row.metadata != null && typeof row.metadata === 'object'
              ? (row.metadata as Record<string, string>)
              : {},
          score: typeof row.score === 'number' ? row.score : 0,
        };
      });
    } finally {
      clearTimeout(timeoutId);
    }
  }

  async delete(id: string): Promise<void> {
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), this.timeout);

    try {
      const resp = await fetch(`${this.base}/v1/memory/${this.store}/${id}`, {
        method: 'DELETE',
        signal: controller.signal,
      });
      if (!resp.ok) throw new Error(`HTTP ${resp.status}: ${await resp.text()}`);
    } finally {
      clearTimeout(timeoutId);
    }
  }
}

// ── GraphStoreClient ──────────────────────────────────────────────────────────

export class GraphStoreClient {
  private readonly base: string;
  private readonly store: string;
  private readonly timeout: number;

  constructor(base: string, store: string, timeout: number) {
    this.base = base;
    this.store = store;
    this.timeout = timeout;
  }

  async addNode(options: AddNodeOptions = {}): Promise<string> {
    const body: Record<string, unknown> = {};
    if (options.labels?.length) body.labels = options.labels;
    if (options.props) body.props = options.props;

    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), this.timeout);

    try {
      const url = options.id
        ? `${this.base}/v1/graph/${this.store}/nodes/${options.id}`
        : `${this.base}/v1/graph/${this.store}/nodes`;
      const method = options.id ? 'PUT' : 'POST';

      const resp = await fetch(url, {
        method,
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
        signal: controller.signal,
      });
      if (!resp.ok) throw new Error(`HTTP ${resp.status}: ${await resp.text()}`);
      const data = (await resp.json()) as Record<string, unknown>;
      return String(data.id ?? '');
    } finally {
      clearTimeout(timeoutId);
    }
  }

  async addEdge(
    fromId: string,
    toId: string,
    relType: string,
    options: AddEdgeOptions = {},
  ): Promise<void> {
    const body: Record<string, unknown> = { from: fromId, to: toId, type: relType };
    if (options.props) body.props = options.props;

    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), this.timeout);

    try {
      const resp = await fetch(`${this.base}/v1/graph/${this.store}/edges`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
        signal: controller.signal,
      });
      if (!resp.ok) throw new Error(`HTTP ${resp.status}: ${await resp.text()}`);
    } finally {
      clearTimeout(timeoutId);
    }
  }

  async cypher(
    query: string,
    params: Record<string, unknown> = {},
  ): Promise<Record<string, unknown>[]> {
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), this.timeout);

    try {
      const resp = await fetch(`${this.base}/v1/graph/${this.store}/cypher`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ query, params }),
        signal: controller.signal,
      });
      if (!resp.ok) throw new Error(`HTTP ${resp.status}: ${await resp.text()}`);
      const data = (await resp.json()) as { rows?: unknown[] };
      return (data.rows ?? []) as Record<string, unknown>[];
    } finally {
      clearTimeout(timeoutId);
    }
  }

  async deleteNode(id: string): Promise<void> {
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), this.timeout);

    try {
      const resp = await fetch(`${this.base}/v1/graph/${this.store}/nodes/${id}`, {
        method: 'DELETE',
        signal: controller.signal,
      });
      if (!resp.ok) throw new Error(`HTTP ${resp.status}: ${await resp.text()}`);
    } finally {
      clearTimeout(timeoutId);
    }
  }
}
