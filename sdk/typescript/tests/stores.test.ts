import { beforeEach, describe, expect, it, vi } from 'vitest';
import { Client } from '../src/client.js';
import type { MemoryResult } from '../src/types.js';

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

// ── MemoryStoreClient ─────────────────────────────────────────────────────────

describe('MemoryStoreClient.upsert', () => {
  it('uses PUT when id is provided', async () => {
    const mock = stubFetchJson({ id: 'doc1' });
    const client = new Client();
    const id = await client.memory('docs').upsert('hello', { id: 'doc1' });

    expect(id).toBe('doc1');
    const [url, init] = mock.mock.calls[0] as [string, RequestInit];
    expect(url).toContain('/v1/memory/docs/doc1');
    expect(init.method).toBe('PUT');
  });

  it('uses POST when id is omitted', async () => {
    const mock = stubFetchJson({ id: 'server-assigned' });
    const client = new Client();
    const id = await client.memory('docs').upsert('hello');

    expect(id).toBe('server-assigned');
    const [url, init] = mock.mock.calls[0] as [string, RequestInit];
    expect(url).toContain('/v1/memory/docs');
    expect((init.method ?? 'POST')).toBe('POST');
  });

  it('sends metadata in request body', async () => {
    const mock = stubFetchJson({ id: 'doc1' });
    const client = new Client();
    await client.memory('docs').upsert('text', { id: 'doc1', metadata: { src: 'wiki' } });

    const [, init] = mock.mock.calls[0] as [string, RequestInit];
    const body = JSON.parse(init.body as string) as Record<string, unknown>;
    expect((body.metadata as Record<string, string>).src).toBe('wiki');
  });
});

describe('MemoryStoreClient.query', () => {
  it('returns parsed MemoryResult array', async () => {
    stubFetchJson({
      results: [
        { id: 'doc1', content: 'Paris', score: 0.9, metadata: { src: 'wiki' } },
        { id: 'doc2', content: 'London', score: 0.7 },
      ],
    });
    const client = new Client();
    const results: MemoryResult[] = await client.memory('docs').query('cities', 2);

    expect(results).toHaveLength(2);
    expect(results[0].id).toBe('doc1');
    expect(results[0].score).toBe(0.9);
    expect(results[0].metadata.src).toBe('wiki');
    expect(results[1].metadata).toEqual({});
  });

  it('sends top_k in request body', async () => {
    const mock = stubFetchJson({ results: [] });
    const client = new Client();
    await client.memory('docs').query('q', 7);

    const [, init] = mock.mock.calls[0] as [string, RequestInit];
    const body = JSON.parse(init.body as string) as Record<string, unknown>;
    expect(body.top_k).toBe(7);
  });

  it('returns empty array when results are absent', async () => {
    stubFetchJson({});
    const client = new Client();
    const results = await client.memory('docs').query('nothing');
    expect(results).toEqual([]);
  });
});

describe('MemoryStoreClient.delete', () => {
  it('sends DELETE request with correct path', async () => {
    const mock = vi.fn().mockResolvedValueOnce({ ok: true, status: 204 });
    vi.stubGlobal('fetch', mock);

    const client = new Client();
    await client.memory('docs').delete('doc1');

    const [url, init] = mock.mock.calls[0] as [string, RequestInit];
    expect(url).toContain('/v1/memory/docs/doc1');
    expect(init.method).toBe('DELETE');
  });
});

// ── GraphStoreClient ──────────────────────────────────────────────────────────

describe('GraphStoreClient.addNode', () => {
  it('uses PUT when id is provided', async () => {
    const mock = stubFetchJson({ id: 'alice' });
    const client = new Client();
    const id = await client.graph('kg').addNode({ id: 'alice', labels: ['Person'] });

    expect(id).toBe('alice');
    const [url, init] = mock.mock.calls[0] as [string, RequestInit];
    expect(url).toContain('/v1/graph/kg/nodes/alice');
    expect(init.method).toBe('PUT');
  });

  it('uses POST when id is omitted', async () => {
    const mock = stubFetchJson({ id: 'generated' });
    const client = new Client();
    const id = await client.graph('kg').addNode({ labels: ['Thing'] });

    expect(id).toBe('generated');
    const [url, init] = mock.mock.calls[0] as [string, RequestInit];
    expect(url).toContain('/v1/graph/kg/nodes');
    expect((init.method ?? 'POST')).toBe('POST');
  });

  it('sends labels and props', async () => {
    const mock = stubFetchJson({ id: 'n1' });
    const client = new Client();
    await client.graph('kg').addNode({ id: 'n1', labels: ['A', 'B'], props: { x: 1 } });

    const [, init] = mock.mock.calls[0] as [string, RequestInit];
    const body = JSON.parse(init.body as string) as Record<string, unknown>;
    expect(body.labels).toEqual(['A', 'B']);
    expect((body.props as Record<string, number>).x).toBe(1);
  });
});

describe('GraphStoreClient.addEdge', () => {
  it('posts correct body', async () => {
    const mock = vi.fn().mockResolvedValueOnce({ ok: true, status: 204 });
    vi.stubGlobal('fetch', mock);

    const client = new Client();
    await client.graph('kg').addEdge('alice', 'bob', 'KNOWS', { props: { since: '2020' } });

    const [url, init] = mock.mock.calls[0] as [string, RequestInit];
    expect(url).toContain('/v1/graph/kg/edges');
    const body = JSON.parse(init.body as string) as Record<string, unknown>;
    expect(body.from).toBe('alice');
    expect(body.to).toBe('bob');
    expect(body.type).toBe('KNOWS');
    expect((body.props as Record<string, string>).since).toBe('2020');
  });
});

describe('GraphStoreClient.cypher', () => {
  it('returns rows from response', async () => {
    stubFetchJson({ rows: [{ 'n.name': 'Alice' }, { 'n.name': 'Bob' }] });
    const client = new Client();
    const rows = await client.graph('kg').cypher('MATCH (n) RETURN n.name');

    expect(rows).toHaveLength(2);
    expect(rows[0]['n.name']).toBe('Alice');
  });

  it('sends params in body', async () => {
    const mock = stubFetchJson({ rows: [] });
    const client = new Client();
    await client.graph('kg').cypher('MATCH (n {id: $id}) RETURN n', { id: 'alice' });

    const [, init] = mock.mock.calls[0] as [string, RequestInit];
    const body = JSON.parse(init.body as string) as Record<string, unknown>;
    expect((body.params as Record<string, string>).id).toBe('alice');
  });

  it('defaults params to empty object', async () => {
    const mock = stubFetchJson({ rows: [] });
    const client = new Client();
    await client.graph('kg').cypher('MATCH (n) RETURN n');

    const [, init] = mock.mock.calls[0] as [string, RequestInit];
    const body = JSON.parse(init.body as string) as Record<string, unknown>;
    expect(body.params).toEqual({});
  });

  it('returns empty array when rows absent', async () => {
    stubFetchJson({});
    const rows = await new Client().graph('kg').cypher('MATCH (n) RETURN n');
    expect(rows).toEqual([]);
  });
});

describe('GraphStoreClient.deleteNode', () => {
  it('sends DELETE with correct path', async () => {
    const mock = vi.fn().mockResolvedValueOnce({ ok: true, status: 204 });
    vi.stubGlobal('fetch', mock);

    const client = new Client();
    await client.graph('kg').deleteNode('alice');

    const [url, init] = mock.mock.calls[0] as [string, RequestInit];
    expect(url).toContain('/v1/graph/kg/nodes/alice');
    expect(init.method).toBe('DELETE');
  });
});
