import { Client } from '../src/index.js';

const client = new Client();

process.stdout.write('Assistant: ');
for await (const text of client.stream('claude', 'Tell me a short joke')) {
  process.stdout.write(text);
}
console.log();
