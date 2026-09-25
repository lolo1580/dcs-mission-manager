// tools/send-telemetry.mjs
//
// Émetteur de télémétrie de test : simule ce que DCS enverrait via Export.lua.
// Permet de valider le backend et la live map sans lancer DCS.
//
// Usage :
//   node tools/send-telemetry.mjs [host] [port]
//   node tools/send-telemetry.mjs 127.0.0.1 7778
//
// L'appareil décrit un cercle autour du Caucase, une position par seconde.

import dgram from 'node:dgram';

const host = process.argv[2] ?? '127.0.0.1';
const port = Number(process.argv[3] ?? 7778);

const socket = dgram.createSocket('udp4');

const center = { lat: 42.0, lng: 41.5 };
const radius = 0.5;
const callsigns = [
  { name: 'Viper 1-1', unitType: 'F-16C_50', coalition: 'blue' },
  { name: 'Hornet 1-2', unitType: 'FA-18C_hornet', coalition: 'blue' },
  { name: 'Flanker 2-1', unitType: 'Su-27', coalition: 'red' },
  { name: 'Hind 3-1', unitType: 'Mi-24P', coalition: 'red' },
];

let t = 0;
console.log(`Envoi de télémétrie vers ${host}:${port} (Ctrl+C pour arrêter)`);

setInterval(() => {
  t += 1;
  for (const [i, c] of callsigns.entries()) {
    const angle = t / 20 + (i * Math.PI) / 2;
    const payload = {
      type: 'ownship',
      name: c.name,
      unitType: c.unitType,
      coalition: c.coalition,
      lat: center.lat + radius * Math.sin(angle) * 0.4,
      lng: center.lng + radius * Math.cos(angle),
      alt: 3000 + 1500 * Math.sin(angle),
      heading: ((angle * 180) / Math.PI + 360) % 360,
      modelTime: t,
    };
    socket.send(JSON.stringify(payload), port, host);
  }
}, 1000);

process.on('SIGINT', () => {
  socket.close();
  process.exit(0);
});
