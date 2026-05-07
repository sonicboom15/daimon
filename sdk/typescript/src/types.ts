export interface MemoryResult {
  id: string;
  content: string;
  metadata: Record<string, string>;
  score: number;
}

export class DaimonError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'DaimonError';
  }
}

export class ToolCall {
  id: string;
  name: string;
  input: Record<string, unknown>;

  constructor(id: string, name: string, input: Record<string, unknown> = {}) {
    this.id = id;
    this.name = name;
    this.input = input;
  }

  toDict(): Record<string, unknown> {
    return { id: this.id, name: this.name, input: this.input };
  }

  static fromDict(obj: Record<string, unknown>): ToolCall {
    const input =
      obj.input != null && typeof obj.input === 'object' && !Array.isArray(obj.input)
        ? (obj.input as Record<string, unknown>)
        : {};
    return new ToolCall(String(obj.id ?? ''), String(obj.name ?? ''), input);
  }
}

export type MessageRole = 'system' | 'user' | 'assistant' | 'tool';

export class Message {
  role: MessageRole;
  content?: string;
  tool_calls?: ToolCall[];
  tool_call_id?: string;

  constructor(
    role: MessageRole,
    content?: string,
    toolCalls?: ToolCall[],
    toolCallId?: string,
  ) {
    this.role = role;
    this.content = content;
    this.tool_calls = toolCalls;
    this.tool_call_id = toolCallId;
  }

  toDict(): Record<string, unknown> {
    const d: Record<string, unknown> = { role: this.role };
    if (this.content) d.content = this.content;
    if (this.tool_calls?.length) d.tool_calls = this.tool_calls.map((tc) => tc.toDict());
    if (this.tool_call_id) d.tool_call_id = this.tool_call_id;
    return d;
  }
}

export class Tool {
  name: string;
  description?: string;
  input_schema?: Record<string, unknown>;

  constructor(name: string, description?: string, inputSchema?: Record<string, unknown>) {
    this.name = name;
    this.description = description;
    this.input_schema = inputSchema;
  }

  toDict(): Record<string, unknown> {
    const d: Record<string, unknown> = {
      name: this.name,
      input_schema: this.input_schema ?? { type: 'object' },
    };
    if (this.description) d.description = this.description;
    return d;
  }
}

export type ChunkType = 'text' | 'tool_call' | 'error' | 'done';

export class Chunk {
  type: ChunkType;
  text: string;
  tool_call?: ToolCall;
  error: string;

  constructor(type: ChunkType, text = '', toolCall?: ToolCall, error = '') {
    this.type = type;
    this.text = text;
    this.tool_call = toolCall;
    this.error = error;
  }

  static fromDict(obj: Record<string, unknown>): Chunk {
    const type = (obj.type as ChunkType) ?? 'text';
    const text = typeof obj.text === 'string' ? obj.text : '';
    const error = typeof obj.error === 'string' ? obj.error : '';

    let toolCall: ToolCall | undefined;
    if (
      obj.tool_call != null &&
      typeof obj.tool_call === 'object' &&
      !Array.isArray(obj.tool_call)
    ) {
      toolCall = ToolCall.fromDict(obj.tool_call as Record<string, unknown>);
    }

    return new Chunk(type, text, toolCall, error);
  }
}
