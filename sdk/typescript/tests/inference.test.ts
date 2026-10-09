import { beforeEach, describe, expect, it, vi } from 'vitest';
import { Client } from '../src/client.js';

function stubFetchJson(body: unknown, status = 200): ReturnType<typeof vi.fn> {
  const mock = vi.fn().mockResolvedValueOnce({
    ok: status >= 200 && status < 300,
    status,
    text: () => Promise.resolve(JSON.stringify(body)),
    json: () => Promise.resolve(body),
  });
  vi.stubGlobal('fetch', mock);
  return mock;
}

beforeEach(() => {
  vi.unstubAllGlobals();
});

// ── NERClient ─────────────────────────────────────────────────────────────────

describe('NERClient.extract', () => {
  it('sends POST to /v1/ner/{model}/extract with text and options', async () => {
    const mock = stubFetchJson({
      entities: [
        {
          text: 'aspirin',
          label: 'medication',
          start: 0,
          end: 7,
          confidence: 0.99,
        },
      ],
    });

    const client = new Client();
    const entities = await client.ner('clinical').extract('aspirin 100mg', {
      labels: ['medication', 'dosage'],
      threshold: 0.8,
    });

    expect(entities).toHaveLength(1);
    expect(entities[0].text).toBe('aspirin');
    expect(entities[0].label).toBe('medication');
    expect(entities[0].start).toBe(0);
    expect(entities[0].end).toBe(7);
    expect(entities[0].confidence).toBe(0.99);

    const [url, init] = mock.mock.calls[0] as [string, RequestInit];
    expect(url).toBe('http://127.0.0.1:3500/v1/ner/clinical/extract');
    expect(init.method).toBe('POST');
    const body = JSON.parse(init.body as string) as Record<string, unknown>;
    expect(body.text).toBe('aspirin 100mg');
    expect(body.labels).toEqual(['medication', 'dosage']);
    expect(body.threshold).toBe(0.8);
  });

  it('omits optional fields when not provided', async () => {
    const mock = stubFetchJson({ entities: [] });
    const client = new Client();
    const entities = await client.ner('default').extract('sample text');

    expect(entities).toEqual([]);
    const [, init] = mock.mock.calls[0] as [string, RequestInit];
    const body = JSON.parse(init.body as string) as Record<string, unknown>;
    expect(body.text).toBe('sample text');
    expect(body.labels).toBeUndefined();
    expect(body.threshold).toBeUndefined();
  });

  it('throws on non-200 HTTP response', async () => {
    stubFetchJson({ error: 'model not found' }, 404);
    const client = new Client();
    await expect(client.ner('missing').extract('text')).rejects.toThrow('HTTP 404');
  });
});

// ── DecisionClient ────────────────────────────────────────────────────────────

describe('DecisionClient.choose', () => {
  it('sends POST to /v1/decision/{model}/choose and parses response', async () => {
    const mock = stubFetchJson({
      selected: 'option A',
      index: 0,
      probabilities: { 'option A': 0.85, 'option B': 0.15 },
    });

    const client = new Client();
    const result = await client.decision('verifier').choose(
      'Which is better?',
      ['option A', 'option B'],
      { state: 'prior context' },
    );

    expect(result.selected).toBe('option A');
    expect(result.index).toBe(0);
    expect(result.probabilities['option A']).toBe(0.85);

    const [url, init] = mock.mock.calls[0] as [string, RequestInit];
    expect(url).toBe('http://127.0.0.1:3500/v1/decision/verifier/choose');
    expect(init.method).toBe('POST');
    const body = JSON.parse(init.body as string) as Record<string, unknown>;
    expect(body.question).toBe('Which is better?');
    expect(body.choices).toEqual(['option A', 'option B']);
    expect(body.state).toBe('prior context');
  });
});

describe('DecisionClient.verify', () => {
  it('sends POST to /v1/decision/{model}/verify and parses response', async () => {
    const mock = stubFetchJson({
      probability: 0.94,
      supported: true,
    });

    const client = new Client();
    const result = await client.decision('verifier').verify(
      'Statement to verify',
      { state: 'evidence context' },
    );

    expect(result.probability).toBe(0.94);
    expect(result.supported).toBe(true);

    const [url, init] = mock.mock.calls[0] as [string, RequestInit];
    expect(url).toBe('http://127.0.0.1:3500/v1/decision/verifier/verify');
    expect(init.method).toBe('POST');
    const body = JSON.parse(init.body as string) as Record<string, unknown>;
    expect(body.statement).toBe('Statement to verify');
    expect(body.state).toBe('evidence context');
  });

  it('throws on HTTP error', async () => {
    stubFetchJson({ error: 'internal error' }, 500);
    const client = new Client();
    await expect(client.decision('verifier').verify('stmt')).rejects.toThrow('HTTP 500');
  });
});

