// tools/send-plain-telemetry.mjs
//
// Minimal telemetry sender used to test source detection end to end. It sends
// an ownship message with a configurable name, plus one world unit, so a session
// can be produced that either looks like the test fixtures or like a real DCS
// player.
//
// Usage:
//   node tools/send-plain-telemetry.mjs <host> <port> <name> <seconds>

import dgram from 'node:dgram';

const host = process.argv[2] ?? '127.0.0.1';
const port = Number(process.argv[3] ?? 7778);
const name = process.argv[4] ?? 'Cellar';
const seconds = Number(process.argv[5] ?? 5);

const socket = dgram.createSocket('udp4');
let t = 0;

const timer = setInterval(() => {
  t += 1;

  socket.send(
    JSON.stringify({
      type: 'ownship',
      name,
      unitType: 'F-16C_50',
      coalition: 'blue',
      country: 'USA',
      lat: 42 + t * 0.001,
      lng: 41.5,
      alt: 3000,
      heading: 90,
      modelTime: t,
    }),
    port,
    host
  );

  if (t % 2 === 0) {
    socket.send(
      JSON.stringify({
        type: 'world',
        count: 1,
        units: [
          { id: '7', type: 'T-72B', coalition: 'red', country: 'Russia', lat: 42, lng: 41, alt: 100, heading: 0 },
        ],
      }),
      port,
      host
    );
  }
}, 1000);

setTimeout(() => {
  clearInterval(timer);
  socket.close();
  console.log(`sent ${t} ownship messages as "${name}"`);
  process.exit(0);
}, seconds * 1000);
