// tools/send-debrief.mjs
//
// Sends a debrief.log to the backend over the TCP channel, just like
// Hooks/dcsmanager.lua would at the end of a mission (chunking + base64).
//
// Usage:
//   node tools/send-debrief.mjs [path] [host] [port]
//   node tools/send-debrief.mjs "%USERPROFILE%\Saved Games\DCS\Logs\debrief.log"
//   node tools/send-debrief.mjs sample.debrief.log 127.0.0.1 7779

import net from 'node:net';
import fs from 'node:fs';
import path from 'node:path';

const DEFAULT_LOG = path.join(
  process.env.USERPROFILE ?? '',
  'Saved Games',
  'DCS',
  'Logs',
  'debrief.log'
);

const file = process.argv[2] ?? DEFAULT_LOG;
const host = process.argv[3] ?? '127.0.0.1';
const port = Number(process.argv[4] ?? 7779);

if (!fs.existsSync(file)) {
  console.error(`File not found: ${file}`);
  console.error('Usage: node tools/send-debrief.mjs [path] [host] [port]');
  process.exit(1);
}

const data = fs.readFileSync(file);
const CHUNK = 32768;
const chunks = Math.ceil(data.length / CHUNK);
const transferId = `cli-${Date.now()}`;
const missionName = path.basename(file);

console.log(`${file} → ${host}:${port} (${data.length} bytes, ${chunks} chunk(s))`);

const socket = net.createConnection({ host, port }, () => {
  for (let i = 0; i < chunks; i++) {
    const part = data.subarray(i * CHUNK, (i + 1) * CHUNK);
    socket.write(
      JSON.stringify({
        type: 'debrief',
        transferId,
        chunk: i,
        chunks,
        size: data.length,
        name: missionName,
        data: part.toString('base64'),
      }) + '\n'
    );
  }
  socket.end();
});

socket.on('close', () => console.log('Sent.'));
socket.on('error', (err) => {
  console.error(`Error: ${err.message}`);
  process.exit(1);
});
