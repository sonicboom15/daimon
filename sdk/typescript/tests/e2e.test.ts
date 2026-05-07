/**
 * End-to-end tests for the TypeScript daimon client against a real daimon sidecar.
 *
 * Requires:
 *   DAIMON_E2E=1         to opt in (the entire suite is skipped otherwise)
 *   DAIMON_BASE_URL=...  to point at the running sidecar
 *
 * Normally invoked by the Go integration test in test/e2e/e2e_test.go, which
 * starts an Ollama container and daimon server automatically.  Can also be run
 * manually against any live sidecar:
 *
 *   DAIMON_E2E=1 DAIMON_BASE_URL=http://127.0.0.1:3500 npm run test:e2e
 */

import { randomBytes } from 'node:crypto';
import { describe, expect, it } from 'vitest';
import { Client, DaimonError } from '../src/index.js';

const COMPONENT = 'llama';
const BASE_URL = process.env.DAIMON_BASE_URL ?? 'http://127.0.0.1:3500';
const MEM_STORE = process.env.DAIMON_MEM_STORE ?? 'mem';
const E2E = Boolean(process.env.DAIMON_E2E);

/** Short random suffix so concurrent test runs don't share session state. */
const sid = () => `vitest-${randomBytes(4).toString('hex')}`;

// Per-test timeout: Ollama on small hardware can be slow.
const TIMEOUT = 120_000;

// ---------------------------------------------------------------------------
// Basic inference
// ---------------------------------------------------------------------------

describe.skipIf(!E2E)('E2E: basic inference', () => {
  const client = new Client({ baseUrl: BASE_URL, timeout: TIMEOUT });

  it('chat returns non-empty text', async () => {
    const result = await client.chat(COMPONENT, 'Reply with exactly one word: PONG');
    expect(result.trim()).not.toBe('');
  }, TIMEOUT);

  it('stream yields text fragments that concatenate to non-empty string', async () => {
    const parts: string[] = [];
    for await (const text of client.stream(COMPONENT, 'Reply with exactly one word: PONG')) {
      parts.push(text);
    }
    expect(parts.length).toBeGreaterThan(0);
    expect(parts.join('').trim()).not.toBe('');
  }, TIMEOUT);

  it('multi-turn without session — client carries history', async () => {
    const messages = [
      { role: 'user', content: 'My favourite colour is red.' },
      { role: 'assistant', content: 'Got it, your favourite colour is red.' },
      { role: 'user', content: 'What colour did I just tell you is my favourite?' },
    ];
    const result = await client.chat(COMPONENT, messages);
    expect(result.toLowerCase()).toContain('red');
  }, TIMEOUT);
});

// ---------------------------------------------------------------------------
// Session management
// ---------------------------------------------------------------------------

describe.skipIf(!E2E)('E2E: session management', () => {
  const client = new Client({ baseUrl: BASE_URL, timeout: TIMEOUT });

  it('session recalls a fact across turns', async () => {
    const sessionId = sid();
    try {
      await client.chat(COMPONENT, 'My favourite colour is blue.', { session_id: sessionId });
      const reply = await client.chat(
        COMPONENT,
        'What colour did I just tell you is my favourite?',
        { session_id: sessionId },
      );
      expect(reply.toLowerCase()).toContain('blue');
    } finally {
      await client.clearSession(sessionId);
    }
  }, TIMEOUT);

  it('session accumulates across stream() calls', async () => {
    const sessionId = sid();
    try {
      // stream() turn 1
      const parts: string[] = [];
      for await (const text of client.stream(
        COMPONENT,
        'My favourite colour is blue.',
        { session_id: sessionId },
      )) {
        parts.push(text);
      }
      expect(parts.join('').trim()).not.toBe('');

      // chat() turn 2 — server should remember the colour
      const reply = await client.chat(
        COMPONENT,
        'What colour did I just tell you is my favourite?',
        { session_id: sessionId },
      );
      expect(reply.toLowerCase()).toContain('blue');
    } finally {
      await client.clearSession(sessionId);
    }
  }, TIMEOUT);

  it('different session IDs are independent', async () => {
    const sidA = sid();
    const sidB = sid();
    try {
      await client.chat(COMPONENT, 'My favourite colour is orange.', { session_id: sidA });
      await client.chat(COMPONENT, 'My favourite colour is green.', { session_id: sidB });

      const replyA = await client.chat(COMPONENT, 'What colour did I just tell you is my favourite?', { session_id: sidA });
      const replyB = await client.chat(COMPONENT, 'What colour did I just tell you is my favourite?', { session_id: sidB });

      expect(replyA.toLowerCase()).toContain('orange');
      expect(replyB.toLowerCase()).toContain('green');
    } finally {
      await client.clearSession(sidA);
      await client.clearSession(sidB);
    }
  }, TIMEOUT);

  it('clearSession resolves without throwing', async () => {
    const sessionId = sid();
    await client.chat(COMPONENT, 'Hello.', { session_id: sessionId });
    await expect(client.clearSession(sessionId)).resolves.toBeUndefined();
  }, TIMEOUT);

  it('clearing a non-existent session does not throw', async () => {
    await expect(client.clearSession(sid())).resolves.toBeUndefined();
  }, TIMEOUT);
});

// ---------------------------------------------------------------------------
// Memory store (inmemory BM25 — no external service)
// ---------------------------------------------------------------------------

describe.skipIf(!E2E)('E2E: memory store', () => {
  const client = new Client({ baseUrl: BASE_URL, timeout: TIMEOUT });

  it('upsert with id returns the same id', async () => {
    const store = client.memory(MEM_STORE);
    const docId = `e2e-ts-${randomBytes(4).toString('hex')}`;
    const returned = await store.upsert('The Eiffel Tower is 330 metres tall.', { id: docId });
    expect(returned).toBe(docId);
  }, TIMEOUT);

  it('upsert without id returns a non-empty server-assigned id', async () => {
    const store = client.memory(MEM_STORE);
    const assigned = await store.upsert('The Seine is the main river of Paris.');
    expect(assigned).not.toBe('');
  }, TIMEOUT);

  it('query returns the seeded document', async () => {
    const store = client.memory(MEM_STORE);
    const docId = `e2e-ts-q-${randomBytes(4).toString('hex')}`;
    await store.upsert('TypeScript is a typed superset of JavaScript.', { id: docId });

    const results = await store.query('typed JavaScript', 5);
    expect(results.length).toBeGreaterThan(0);

    const ids = results.map((r) => r.id);
    expect(ids).toContain(docId);
  }, TIMEOUT);

  it('query results are MemoryResult-shaped', async () => {
    const store = client.memory(MEM_STORE);
    const docId = `e2e-ts-shape-${randomBytes(4).toString('hex')}`;
    await store.upsert('Go is a statically typed compiled language.', { id: docId });

    const results = await store.query('compiled language', 3);
    expect(results.length).toBeGreaterThan(0);
    for (const r of results) {
      expect(typeof r.id).toBe('string');
      expect(typeof r.content).toBe('string');
      expect(typeof r.score).toBe('number');
      expect(typeof r.metadata).toBe('object');
    }
  }, TIMEOUT);

  it('upsert with metadata stores the document', async () => {
    const store = client.memory(MEM_STORE);
    const docId = `e2e-ts-meta-${randomBytes(4).toString('hex')}`;
    await store.upsert('Rust prevents memory safety bugs.', { id: docId, metadata: { src: 'wiki' } });

    const results = await store.query('memory safety', 5);
    const ids = results.map((r) => r.id);
    expect(ids).toContain(docId);
  }, TIMEOUT);

  it('delete does not throw', async () => {
    const store = client.memory(MEM_STORE);
    const docId = `e2e-ts-del-${randomBytes(4).toString('hex')}`;
    await store.upsert('Temporary document.', { id: docId });
    await expect(store.delete(docId)).resolves.toBeUndefined();
  }, TIMEOUT);

  it('double delete is idempotent', async () => {
    const store = client.memory(MEM_STORE);
    const docId = `e2e-ts-idem-${randomBytes(4).toString('hex')}`;
    await store.upsert('Another temporary document.', { id: docId });
    await store.delete(docId);
    await expect(store.delete(docId)).resolves.toBeUndefined();
  }, TIMEOUT);
});

// ---------------------------------------------------------------------------
// Error handling
// ---------------------------------------------------------------------------

describe.skipIf(!E2E)('E2E: error handling', () => {
  const client = new Client({ baseUrl: BASE_URL, timeout: TIMEOUT });

  it('unknown component returns DaimonError', async () => {
    await expect(
      client.chat('does-not-exist', 'hello'),
    ).rejects.toThrow(DaimonError);
  }, TIMEOUT);
});
