import { beforeEach, describe, expect, it, vi } from 'vitest';
import { Client, LLMClient } from '../src/client.js';
import { DaimonError, ToolCall } from '../src/types.js';

function makeSSEStream(events: string[]): ReadableStream<Uint8Array> {
  const encoder = new TextEncoder();
  const frames = events.map((e) => encoder.encode(`data: ${e}\n\n`));
  let index = 0;
  return new ReadableStream({
    pull(controller) {
      if (index < frames.length) {
        controller.enqueue(frames[index++]);
      } else {
        controller.close();
      }
    },
  });
}

function stubFetch(stream: ReadableStream, status = 200): ReturnType<typeof vi.fn> {
  const mock = vi.fn().mockResolvedValueOnce({
    ok: status >= 200 && status < 300,
    status,
    text: () => Promise.resolve('error body'),
    body: stream,
  });
  vi.stubGlobal('fetch', mock);
  return mock;
}

beforeEach(() => {
  vi.unstubAllGlobals();
});

// ---------------------------------------------------------------------------
// converse
// ---------------------------------------------------------------------------

describe('Client.converse', () => {
  it('yields all chunks including done', async () => {
    const stream = makeSSEStream([
      JSON.stringify({ type: 'text', text: 'Hello' }),
      JSON.stringify({ type: 'text', text: ' world' }),
      JSON.stringify({ type: 'done' }),
    ]);
    stubFetch(stream);

    const client = new Client();
    const chunks = [];
    for await (const chunk of client.converse('claude', {
      messages: [{ role: 'user', content: 'hi' }],
    })) {
      chunks.push(chunk);
    }

    expect(chunks).toHaveLength(3);
    expect(chunks[0].type).toBe('text');
    expect(chunks[0].text).toBe('Hello');
    expect(chunks[1].text).toBe(' world');
    expect(chunks[2].type).toBe('done');
  });

  it('stops after first error chunk', async () => {
    const stream = makeSSEStream([
      JSON.stringify({ type: 'error', error: 'provider failed' }),
      JSON.stringify({ type: 'text', text: 'never' }),
    ]);
    stubFetch(stream);

    const client = new Client();
    const chunks = [];
    for await (const chunk of client.converse('claude', { messages: [] })) {
      chunks.push(chunk);
    }

    expect(chunks).toHaveLength(1);
    expect(chunks[0].type).toBe('error');
    expect(chunks[0].error).toBe('provider failed');
  });

  it('throws DaimonError on non-ok HTTP status', async () => {
    stubFetch(makeSSEStream([]), 404);

    const client = new Client();
    await expect(async () => {
      for await (const _ of client.converse('unknown', { messages: [] })) {
        // noop
      }
    }).rejects.toThrow(DaimonError);
  });

  it('throws DaimonError on 500', async () => {
    stubFetch(makeSSEStream([]), 500);

    const client = new Client();
    await expect(async () => {
      for await (const _ of client.converse('claude', { messages: [] })) {
        // noop
      }
    }).rejects.toThrow('HTTP 500');
  });

  it('skips SSE comment lines', async () => {
    const encoder = new TextEncoder();
    const body = [
      ': keep-alive\n\n',
      `data: ${JSON.stringify({ type: 'text', text: 'hi' })}\n\n`,
      `data: ${JSON.stringify({ type: 'done' })}\n\n`,
    ].join('');

    const stream = new ReadableStream<Uint8Array>({
      start(controller) {
        controller.enqueue(encoder.encode(body));
        controller.close();
      },
    });
    stubFetch(stream);

    const client = new Client();
    const chunks = [];
    for await (const chunk of client.converse('claude', { messages: [] })) {
      chunks.push(chunk);
    }

    expect(chunks[0].type).toBe('text');
    expect(chunks[0].text).toBe('hi');
  });

  it('sends correct request body', async () => {
    const mock = stubFetch(makeSSEStream([JSON.stringify({ type: 'done' })]));

    const client = new Client();
    for await (const _ of client.converse('claude', {
      messages: [{ role: 'user', content: 'hello' }],
      model: 'claude-haiku-4-5',
      temperature: 0.7,
      max_tokens: 100,
    })) {
      // noop
    }

    const [url, init] = mock.mock.calls[0] as [string, RequestInit];
    expect(url).toBe('http://127.0.0.1:3500/v1/converse/claude');
    const body = JSON.parse(init.body as string);
    expect(body.messages).toEqual([{ role: 'user', content: 'hello' }]);
    expect(body.model).toBe('claude-haiku-4-5');
    expect(body.temperature).toBe(0.7);
    expect(body.max_tokens).toBe(100);
  });

  it('omits undefined optional fields from request body', async () => {
    const mock = stubFetch(makeSSEStream([JSON.stringify({ type: 'done' })]));

    const client = new Client();
    for await (const _ of client.converse('claude', {
      messages: [{ role: 'user', content: 'hi' }],
    })) {
      // noop
    }

    const body = JSON.parse((mock.mock.calls[0] as [string, RequestInit])[1].body as string);
    expect(body).not.toHaveProperty('model');
    expect(body).not.toHaveProperty('temperature');
    expect(body).not.toHaveProperty('tools');
  });

  it('strips trailing slash from baseUrl', async () => {
    const mock = stubFetch(makeSSEStream([JSON.stringify({ type: 'done' })]));

    const client = new Client({ baseUrl: 'http://127.0.0.1:3500/' });
    for await (const _ of client.converse('claude', { messages: [] })) {
      // noop
    }

    const url = (mock.mock.calls[0] as [string])[0];
    expect(url).toBe('http://127.0.0.1:3500/v1/converse/claude');
  });
});

// ---------------------------------------------------------------------------
// stream
// ---------------------------------------------------------------------------

describe('Client.stream', () => {
  it('yields text from text chunks only', async () => {
    const stream = makeSSEStream([
      JSON.stringify({ type: 'text', text: 'Hello' }),
      JSON.stringify({ type: 'text', text: ' world' }),
      JSON.stringify({ type: 'done' }),
    ]);
    stubFetch(stream);

    const texts: string[] = [];
    const client = new Client();
    for await (const text of client.stream('claude', 'hi')) {
      texts.push(text);
    }

    expect(texts).toEqual(['Hello', ' world']);
  });

  it('normalizes string prompt to user message', async () => {
    const mock = stubFetch(makeSSEStream([JSON.stringify({ type: 'done' })]));

    const client = new Client();
    for await (const _ of client.stream('claude', 'hello world')) {
      // noop
    }

    const body = JSON.parse((mock.mock.calls[0] as [string, RequestInit])[1].body as string);
    expect(body.messages).toEqual([{ role: 'user', content: 'hello world' }]);
  });

  it('passes message array through unchanged', async () => {
    const mock = stubFetch(makeSSEStream([JSON.stringify({ type: 'done' })]));

    const client = new Client();
    const messages = [
      { role: 'system', content: 'Be helpful' },
      { role: 'user', content: 'hi' },
    ];
    for await (const _ of client.stream('claude', messages)) {
      // noop
    }

    const body = JSON.parse((mock.mock.calls[0] as [string, RequestInit])[1].body as string);
    expect(body.messages).toEqual(messages);
  });

  it('calls onToolCall for tool_call chunks', async () => {
    const stream = makeSSEStream([
      JSON.stringify({
        type: 'tool_call',
        tool_call: { id: 'tc1', name: 'get_weather', input: { city: 'Tokyo' } },
      }),
      JSON.stringify({ type: 'done' }),
    ]);
    stubFetch(stream);

    const toolCalls: ToolCall[] = [];
    const client = new Client();
    for await (const _ of client.stream('claude', 'hi', {
      onToolCall: (tc) => toolCalls.push(tc),
    })) {
      // noop
    }

    expect(toolCalls).toHaveLength(1);
    expect(toolCalls[0].name).toBe('get_weather');
    expect(toolCalls[0].input).toEqual({ city: 'Tokyo' });
  });

  it('silently skips tool_call chunks when no callback provided', async () => {
    const stream = makeSSEStream([
      JSON.stringify({
        type: 'tool_call',
        tool_call: { id: 'tc1', name: 'get_weather', input: {} },
      }),
      JSON.stringify({ type: 'text', text: 'It is sunny.' }),
      JSON.stringify({ type: 'done' }),
    ]);
    stubFetch(stream);

    const texts: string[] = [];
    const client = new Client();
    for await (const text of client.stream('claude', 'hi')) {
      texts.push(text);
    }

    expect(texts).toEqual(['It is sunny.']);
  });

  it('throws DaimonError on error chunk', async () => {
    stubFetch(
      makeSSEStream([JSON.stringify({ type: 'error', error: 'provider failed' })]),
    );

    const client = new Client();
    await expect(async () => {
      for await (const _ of client.stream('claude', 'hi')) {
        // noop
      }
    }).rejects.toThrow(DaimonError);
  });

  it('throws DaimonError with error message', async () => {
    stubFetch(
      makeSSEStream([JSON.stringify({ type: 'error', error: 'rate limit exceeded' })]),
    );

    const client = new Client();
    await expect(async () => {
      for await (const _ of client.stream('claude', 'hi')) {
        // noop
      }
    }).rejects.toThrow('rate limit exceeded');
  });
});

// ---------------------------------------------------------------------------
// session_id
// ---------------------------------------------------------------------------

describe('session_id', () => {
  it('is included in request body when provided', async () => {
    const mock = stubFetch(makeSSEStream([JSON.stringify({ type: 'done' })]));

    const client = new Client();
    for await (const _ of client.converse('claude', {
      messages: [{ role: 'user', content: 'hi' }],
      session_id: 'sess-abc',
    })) {
      // noop
    }

    const body = JSON.parse((mock.mock.calls[0] as [string, RequestInit])[1].body as string);
    expect(body.session_id).toBe('sess-abc');
  });

  it('is omitted from body when not provided', async () => {
    const mock = stubFetch(makeSSEStream([JSON.stringify({ type: 'done' })]));

    const client = new Client();
    for await (const _ of client.converse('claude', {
      messages: [{ role: 'user', content: 'hi' }],
    })) {
      // noop
    }

    const body = JSON.parse((mock.mock.calls[0] as [string, RequestInit])[1].body as string);
    expect(body).not.toHaveProperty('session_id');
  });

  it('flows through stream() options', async () => {
    const mock = stubFetch(
      makeSSEStream([
        JSON.stringify({ type: 'text', text: 'hi' }),
        JSON.stringify({ type: 'done' }),
      ]),
    );

    const client = new Client();
    for await (const _ of client.stream('claude', 'hello', { session_id: 'sess-xyz' })) {
      // noop
    }

    const body = JSON.parse((mock.mock.calls[0] as [string, RequestInit])[1].body as string);
    expect(body.session_id).toBe('sess-xyz');
  });
});

// ---------------------------------------------------------------------------
// clearSession
// ---------------------------------------------------------------------------

describe('Client.clearSession', () => {
  it('sends DELETE to /v1/sessions/{id}', async () => {
    const mock = vi.fn().mockResolvedValueOnce({
      ok: true,
      status: 204,
      text: () => Promise.resolve(''),
    });
    vi.stubGlobal('fetch', mock);

    const client = new Client();
    await client.clearSession('sess-abc');

    const [url, init] = mock.mock.calls[0] as [string, RequestInit];
    expect(url).toBe('http://127.0.0.1:3500/v1/sessions/sess-abc');
    expect(init.method).toBe('DELETE');
  });

  it('throws DaimonError on non-ok response', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValueOnce({
        ok: false,
        status: 404,
        text: () => Promise.resolve('not found'),
      }),
    );

    const client = new Client();
    await expect(client.clearSession('nonexistent')).rejects.toThrow(DaimonError);
  });
});

// ---------------------------------------------------------------------------
// LLMClient
// ---------------------------------------------------------------------------

describe('LLMClient (via client.llm)', () => {
  it('routes to the named component URL', async () => {
    const mock = stubFetch(makeSSEStream([JSON.stringify({ type: 'done' })]));
    const client = new Client();
    await client.llm('claude').chat('hi');
    const url = (mock.mock.calls[0] as [string])[0];
    expect(url).toBe('http://127.0.0.1:3500/v1/converse/claude');
  });

  it('defaults to "default" component when no name given', async () => {
    const mock = stubFetch(makeSSEStream([JSON.stringify({ type: 'done' })]));
    const client = new Client();
    await client.llm().chat('hi');
    const url = (mock.mock.calls[0] as [string])[0];
    expect(url).toBe('http://127.0.0.1:3500/v1/converse/default');
  });

  it('chat returns joined text', async () => {
    stubFetch(makeSSEStream([
      JSON.stringify({ type: 'text', text: 'Hello' }),
      JSON.stringify({ type: 'text', text: ' world' }),
      JSON.stringify({ type: 'done' }),
    ]));
    const result = await new Client().llm('claude').chat('hi');
    expect(result).toBe('Hello world');
  });

  it('stream yields text fragments', async () => {
    stubFetch(makeSSEStream([
      JSON.stringify({ type: 'text', text: 'A' }),
      JSON.stringify({ type: 'text', text: 'B' }),
      JSON.stringify({ type: 'done' }),
    ]));
    const texts: string[] = [];
    for await (const t of new Client().llm('claude').stream('hi')) {
      texts.push(t);
    }
    expect(texts).toEqual(['A', 'B']);
  });

  it('converse yields raw chunks', async () => {
    stubFetch(makeSSEStream([
      JSON.stringify({ type: 'text', text: 'hi' }),
      JSON.stringify({ type: 'done' }),
    ]));
    const chunks = [];
    for await (const c of new Client().llm('claude').converse({ messages: [{ role: 'user', content: 'hi' }] })) {
      chunks.push(c);
    }
    expect(chunks.map((c) => c.type)).toEqual(['text', 'done']);
  });

  it('clearSession sends DELETE to correct path', async () => {
    const mock = vi.fn().mockResolvedValueOnce({ ok: true, status: 204, text: () => Promise.resolve('') });
    vi.stubGlobal('fetch', mock);
    await new Client().llm('claude').clearSession('sess-abc');
    const [url, init] = mock.mock.calls[0] as [string, RequestInit];
    expect(url).toBe('http://127.0.0.1:3500/v1/sessions/sess-abc');
    expect(init.method).toBe('DELETE');
  });

  it('throws DaimonError on error chunk', async () => {
    stubFetch(makeSSEStream([JSON.stringify({ type: 'error', error: 'boom' })]));
    await expect(async () => {
      for await (const _ of new Client().llm('claude').stream('hi')) { /* noop */ }
    }).rejects.toThrow('boom');
  });

  it('can be instantiated standalone', async () => {
    const mock = stubFetch(makeSSEStream([
      JSON.stringify({ type: 'text', text: 'ok' }),
      JSON.stringify({ type: 'done' }),
    ]));
    const llm = new LLMClient('http://127.0.0.1:3500', 'my-llm', 5000);
    const result = await llm.chat('hi');
    expect(result).toBe('ok');
    const url = (mock.mock.calls[0] as [string])[0];
    expect(url).toContain('/v1/converse/my-llm');
  });
});

// ---------------------------------------------------------------------------
// chat
// ---------------------------------------------------------------------------

describe('Client.chat', () => {
  it('returns joined text response', async () => {
    stubFetch(
      makeSSEStream([
        JSON.stringify({ type: 'text', text: 'Hello' }),
        JSON.stringify({ type: 'text', text: ' world' }),
        JSON.stringify({ type: 'done' }),
      ]),
    );

    const client = new Client();
    const result = await client.chat('claude', 'hi');
    expect(result).toBe('Hello world');
  });

  it('returns empty string for done-only stream', async () => {
    stubFetch(makeSSEStream([JSON.stringify({ type: 'done' })]));

    const client = new Client();
    const result = await client.chat('claude', 'hi');
    expect(result).toBe('');
  });

  it('throws DaimonError on error chunk', async () => {
    stubFetch(
      makeSSEStream([JSON.stringify({ type: 'error', error: 'something broke' })]),
    );

    const client = new Client();
    await expect(client.chat('claude', 'hi')).rejects.toThrow(DaimonError);
  });

  it('forwards inference params to request body', async () => {
    const mock = stubFetch(makeSSEStream([JSON.stringify({ type: 'done' })]));

    const client = new Client();
    await client.chat('claude', 'hi', { temperature: 0.5, top_k: 40 });

    const body = JSON.parse((mock.mock.calls[0] as [string, RequestInit])[1].body as string);
    expect(body.temperature).toBe(0.5);
    expect(body.top_k).toBe(40);
  });
});
