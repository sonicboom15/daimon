import { Client, Message } from '../src/index.js';

const client = new Client();

const messages: Message[] = [
  new Message('user', "What's the weather in Tokyo and Paris?"),
];

process.stdout.write('Assistant: ');
for await (const text of client.stream('claude', messages, {
  onToolCall: (tc) => {
    console.log(`\n[Tool: ${tc.name}(${JSON.stringify(tc.input)})]`);
    process.stdout.write('Assistant: ');
  },
})) {
  process.stdout.write(text);
}
console.log();
