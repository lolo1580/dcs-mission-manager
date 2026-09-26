// tools/send-debrief.mjs
//
// Envoie un debrief.log au backend via le canal TCP, comme le ferait
// Hooks/dcsmm.lua à la fin d'une mission (découpage + base64).
//
// Usage :
//   node tools/send-debrief.mjs [chemin] [host] [port]
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
  console.error(`Fichier introuvable : ${file}`);
  console.error('Usage : node tools/send-debrief.mjs [chemin] [host] [port]');
  process.exit(1);
}

const data = fs.readFileSync(file);
const CHUNK = 32768;
const chunks = Math.ceil(data.length / CHUNK);
const transferId = `cli-${Date.now()}`;
const missionName = path.basename(file);

console.log(`${file} → ${host}:${port} (${data.length} octets, ${chunks} morceau(x))`);

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

socket.on('close', () => console.log('Envoyé.'));
socket.on('error', (err) => {
  console.error(`Erreur : ${err.message}`);
  process.exit(1);
});
