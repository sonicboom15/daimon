import { Chunk, DaimonError, Message, Tool, ToolCall } from './types.js';

export type MessageLike = Message | Record<string, unknown>;
export type ToolLike = Tool | Record<string, unknown>;

export interface ConversationOptions {
  messages: MessageLike[];
  model?: string;
  system?: string;
  tools?: ToolLike[];
  max_tokens?: number;
  temperature?: number;
  top_p?: number;
  top_k?: number;
  stop?: string[];
  frequency_penalty?: number;
  presence_penalty?: number;
  seed?: number;
  session_id?: string;
}

export interface StreamOptions {
  onToolCall?: (toolCall: ToolCall) => void;
  model?: string;
  system?: string;
  tools?: ToolLike[];
  max_tokens?: number;
  temperature?: number;
  top_p?: number;
  top_k?: number;
  stop?: string[];
  frequency_penalty?: number;
  presence_penalty?: number;
  seed?: number;
  session_id?: string;
}

export type ChatOptions = Omit<StreamOptions, 'onToolCall'>;

export interface ClientOptions {
  baseUrl?: string;
  timeout?: number;
}

function serializeMessage(m: MessageLike): Record<string, unknown> {
  return m instanceof Message ? m.toDict() : m;
}

function serializeTool(t: ToolLike): Record<string, unknown> {
  return t instanceof Tool ? t.toDict() : t;
}

function buildBody(
  messages: MessageLike[],
  options: Omit<ConversationOptions, 'messages'>,
): Record<string, unknown> {
  const body: Record<string, unknown> = {
    messages: messages.map(serializeMessage),
  };
  if (options.model != null) body.model = options.model;
  if (options.system != null) body.system = options.system;
  if (options.tools?.length) body.tools = options.tools.map(serializeTool);
  if (options.max_tokens != null) body.max_tokens = options.max_tokens;
  if (options.temperature != null) body.temperature = options.temperature;
  if (options.top_p != null) body.top_p = options.top_p;
  if (options.top_k != null) body.top_k = options.top_k;
  if (options.stop?.length) body.stop = options.stop;
  if (options.frequency_penalty != null) body.frequency_penalty = options.frequency_penalty;
  if (options.presence_penalty != null) body.presence_penalty = options.presence_penalty;
  if (options.seed != null) body.seed = options.seed;
  if (options.session_id != null) body.session_id = options.session_id;
  return body;
}

function normalizeInput(promptOrMessages: string | MessageLike[]): MessageLike[] {
  if (typeof promptOrMessages === 'string') {
    return [{ role: 'user', content: promptOrMessages }];
  }
  return promptOrMessages;
}

async function* parseSSE(body: ReadableStream<Uint8Array>): AsyncGenerator<Chunk> {
  const decoder = new TextDecoder();
  const reader = body.getReader();
  let buffer = '';

  try {
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;

      buffer += decoder.decode(value, { stream: true });
      const lines = buffer.split('\n');
      buffer = lines.pop() ?? '';

      for (const line of lines) {
        const trimmed = line.trimEnd();
        if (trimmed === '' || trimmed.startsWith(':')) continue;
        if (!trimmed.startsWith('data: ')) continue;

        const chunk = Chunk.fromDict(JSON.parse(trimmed.slice(6)) as Record<string, unknown>);
        yield chunk;
        if (chunk.type === 'done' || chunk.type === 'error') return;
      }
    }

    // flush any partial last line (no trailing newline)
    const trimmed = buffer.trimEnd();
    if (trimmed.startsWith('data: ')) {
      yield Chunk.fromDict(JSON.parse(trimmed.slice(6)) as Record<string, unknown>);
    }
  } finally {
    reader.releaseLock();
  }
}

export class Client {
  private readonly baseUrl: string;
  private readonly timeout: number;

  constructor(options: ClientOptions = {}) {
    this.baseUrl = (options.baseUrl ?? 'http://127.0.0.1:3500').replace(/\/$/, '');
    this.timeout = options.timeout ?? 120_000;
  }

  async *converse(component: string, options: ConversationOptions): AsyncGenerator<Chunk> {
    const { messages, ...rest } = options;
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), this.timeout);

    try {
      const response = await fetch(`${this.baseUrl}/v1/converse/${component}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(buildBody(messages, rest)),
        signal: controller.signal,
      });

      if (!response.ok) {
        throw new DaimonError(`HTTP ${response.status}: ${await response.text()}`);
      }
      if (!response.body) {
        throw new DaimonError('No response body');
      }

      yield* parseSSE(response.body);
    } finally {
      clearTimeout(timeoutId);
    }
  }

  async *stream(
    component: string,
    promptOrMessages: string | MessageLike[],
    options: StreamOptions = {},
  ): AsyncGenerator<string> {
    const { onToolCall, ...rest } = options;
    const messages = normalizeInput(promptOrMessages);

    for await (const chunk of this.converse(component, { messages, ...rest })) {
      if (chunk.type === 'text') {
        yield chunk.text;
      } else if (chunk.type === 'tool_call' && onToolCall != null && chunk.tool_call != null) {
        onToolCall(chunk.tool_call);
      } else if (chunk.type === 'error') {
        throw new DaimonError(chunk.error || 'Unknown error');
      }
    }
  }

  async chat(
    component: string,
    promptOrMessages: string | MessageLike[],
    options: ChatOptions = {},
  ): Promise<string> {
    const parts: string[] = [];
    for await (const text of this.stream(component, promptOrMessages, options)) {
      parts.push(text);
    }
    return parts.join('');
  }

  async clearSession(sessionId: string): Promise<void> {
    const response = await fetch(`${this.baseUrl}/v1/sessions/${sessionId}`, {
      method: 'DELETE',
    });
    if (!response.ok) {
      throw new DaimonError(`HTTP ${response.status}: ${await response.text()}`);
    }
  }
}
