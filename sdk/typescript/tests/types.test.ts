import { describe, expect, it } from 'vitest';
import { Chunk, DaimonError, Message, Tool, ToolCall } from '../src/types.js';

describe('ToolCall', () => {
  it('constructs with defaults', () => {
    const tc = new ToolCall('id1', 'my_tool');
    expect(tc.id).toBe('id1');
    expect(tc.name).toBe('my_tool');
    expect(tc.input).toEqual({});
  });

  it('serializes via toDict', () => {
    const tc = new ToolCall('id1', 'my_tool', { city: 'Tokyo' });
    expect(tc.toDict()).toEqual({ id: 'id1', name: 'my_tool', input: { city: 'Tokyo' } });
  });

  it('deserializes via fromDict', () => {
    const tc = ToolCall.fromDict({ id: 'id1', name: 'my_tool', input: { city: 'Tokyo' } });
    expect(tc.id).toBe('id1');
    expect(tc.name).toBe('my_tool');
    expect(tc.input).toEqual({ city: 'Tokyo' });
  });

  it('defaults missing input to empty object', () => {
    const tc = ToolCall.fromDict({ id: 'id1', name: 'my_tool' });
    expect(tc.input).toEqual({});
  });

  it('defaults array input to empty object', () => {
    const tc = ToolCall.fromDict({ id: 'id1', name: 'my_tool', input: [1, 2, 3] });
    expect(tc.input).toEqual({});
  });

  it('coerces numeric id to string', () => {
    const tc = ToolCall.fromDict({ id: 42, name: 'tool' });
    expect(tc.id).toBe('42');
  });

  it('defaults null name to empty string', () => {
    const tc = ToolCall.fromDict({ id: 'id1', name: null });
    expect(tc.name).toBe('');
  });
});

describe('Message', () => {
  it('omits undefined fields in toDict', () => {
    const m = new Message('user', 'hello');
    expect(m.toDict()).toEqual({ role: 'user', content: 'hello' });
  });

  it('omits empty content', () => {
    const m = new Message('assistant');
    expect(m.toDict()).toEqual({ role: 'assistant' });
  });

  it('includes tool_calls when present', () => {
    const tc = new ToolCall('id1', 'my_tool', {});
    const m = new Message('assistant', undefined, [tc]);
    expect(m.toDict()).toEqual({
      role: 'assistant',
      tool_calls: [{ id: 'id1', name: 'my_tool', input: {} }],
    });
  });

  it('omits empty tool_calls array', () => {
    const m = new Message('assistant', 'hi', []);
    expect(m.toDict()).not.toHaveProperty('tool_calls');
  });

  it('includes tool_call_id when present', () => {
    const m = new Message('tool', 'result', undefined, 'call_123');
    expect(m.toDict()).toEqual({
      role: 'tool',
      content: 'result',
      tool_call_id: 'call_123',
    });
  });
});

describe('Tool', () => {
  it('defaults input_schema to object type', () => {
    const t = new Tool('my_tool');
    expect(t.toDict()).toEqual({ name: 'my_tool', input_schema: { type: 'object' } });
  });

  it('includes description when provided', () => {
    const t = new Tool('my_tool', 'Does something');
    expect(t.toDict()).toEqual({
      name: 'my_tool',
      description: 'Does something',
      input_schema: { type: 'object' },
    });
  });

  it('uses provided input_schema', () => {
    const schema = { type: 'object', properties: { city: { type: 'string' } } };
    const t = new Tool('get_weather', 'Get weather', schema);
    expect(t.toDict()).toEqual({
      name: 'get_weather',
      description: 'Get weather',
      input_schema: schema,
    });
  });
});

describe('Chunk', () => {
  it('parses text chunk', () => {
    const c = Chunk.fromDict({ type: 'text', text: 'hello' });
    expect(c.type).toBe('text');
    expect(c.text).toBe('hello');
  });

  it('parses done chunk', () => {
    const c = Chunk.fromDict({ type: 'done' });
    expect(c.type).toBe('done');
    expect(c.text).toBe('');
    expect(c.error).toBe('');
  });

  it('parses error chunk', () => {
    const c = Chunk.fromDict({ type: 'error', error: 'provider failed' });
    expect(c.type).toBe('error');
    expect(c.error).toBe('provider failed');
  });

  it('parses tool_call chunk', () => {
    const c = Chunk.fromDict({
      type: 'tool_call',
      tool_call: { id: 'id1', name: 'get_weather', input: { city: 'Tokyo' } },
    });
    expect(c.type).toBe('tool_call');
    expect(c.tool_call?.name).toBe('get_weather');
    expect(c.tool_call?.input).toEqual({ city: 'Tokyo' });
  });

  it('defaults missing text to empty string', () => {
    const c = Chunk.fromDict({ type: 'text' });
    expect(c.text).toBe('');
  });

  it('leaves tool_call undefined for non-object value', () => {
    const c = Chunk.fromDict({ type: 'tool_call', tool_call: 'not an object' });
    expect(c.tool_call).toBeUndefined();
  });

  it('leaves tool_call undefined for array value', () => {
    const c = Chunk.fromDict({ type: 'tool_call', tool_call: [1, 2] });
    expect(c.tool_call).toBeUndefined();
  });
});

describe('DaimonError', () => {
  it('extends Error', () => {
    const e = new DaimonError('something went wrong');
    expect(e).toBeInstanceOf(Error);
    expect(e.message).toBe('something went wrong');
    expect(e.name).toBe('DaimonError');
  });
});
