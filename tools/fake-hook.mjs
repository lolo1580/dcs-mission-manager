// Fake DCS hook: connects to the backend's TCP port, and behaves like
// Hooks/dcsmanager.lua — it stays connected and prints any command the backend
// pushes. Used to test the command channel without launching DCS.
//
// Usage: node tools/fake-hook.mjs [host] [port] [seconds]
import net from 'node:net';

const host = process.argv[2] ?? '127.0.0.1';
const port = Number(process.argv[3] ?? 7779);
const seconds = Number(process.argv[4] ?? 30);

const sock = net.createConnection({ host, port }, () => {
  console.log(`fake-hook: connected to ${host}:${port}`);
  // Announce a mission start, exactly like the real hook does.
  sock.write(JSON.stringify({ type: 'mission', phase: 'start', name: 'Fake mission' }) + '\n');
});

let buf = '';
sock.on('data', (d) => {
  buf += d.toString('utf8');
  let nl;
  while ((nl = buf.indexOf('\n')) >= 0) {
    const line = buf.slice(0, nl);
    buf = buf.slice(nl + 1);
    if (!line) continue;
    try {
      const msg = JSON.parse(line);
      if (msg.type === 'command') {
        console.log(`fake-hook: COMMAND ${msg.command} from ${msg.from ?? '?'}: ${msg.message}`);
      } else {
        console.log(`fake-hook: ${line}`);
      }
    } catch {
      console.log(`fake-hook: non-JSON: ${line}`);
    }
  }
});

sock.on('error', (e) => console.log(`fake-hook: ${e.message}`));
sock.on('close', () => console.log('fake-hook: closed'));

setTimeout(() => {
  sock.end();
  process.exit(0);
}, seconds * 1000);
