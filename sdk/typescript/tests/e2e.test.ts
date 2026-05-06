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
