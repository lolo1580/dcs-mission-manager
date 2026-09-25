// tools/send-telemetry.mjs
//
// Émetteur de télémétrie de test : simule ce que DCS enverrait via Export.lua.
// Permet de valider le backend et la live map sans lancer DCS.
//
// Usage :
//   node tools/send-telemetry.mjs [host] [port]
//   node tools/send-telemetry.mjs 127.0.0.1 7778
//
// Émet :
//   - un message "ownship" par seconde (le joueur) ;
//   - un message "world" toutes les 2 secondes (unités IA + navires + sol).

import dgram from 'node:dgram';

const host = process.argv[2] ?? '127.0.0.1';
const port = Number(process.argv[3] ?? 7778);

const socket = dgram.createSocket('udp4');

const center = { lat: 42.0, lng: 41.5 };
const radius = 0.5;

const air = [
  { name: 'Viper 1-1', unitType: 'F-16C_50', coalition: 'blue' },
  { name: 'Hornet 1-2', unitType: 'FA-18C_hornet', coalition: 'blue' },
  { name: 'Flanker 2-1', unitType: 'Su-27', coalition: 'red' },
  { name: 'Hind 3-1', unitType: 'Mi-24P', coalition: 'red' },
];

const worldTemplates = [
  { type: 'T-72B', coalition: 'red', country: 'Russia' },
  { type: 'BTR-80', coalition: 'red', country: 'Russia' },
  { type: 'SA-10', coalition: 'red', country: 'Russia' },
  { type: 'Ural-375', coalition: 'red', country: 'Russia' },
  { type: 'M1A2', coalition: 'blue', country: 'USA' },
  { type: 'M2A2 Bradley', coalition: 'blue', country: 'USA' },
  { type: 'Patriot', coalition: 'blue', country: 'USA' },
  { type: 'HMMWV', coalition: 'blue', country: 'USA' },
  { type: 'USS_Arleigh_Burke', coalition: 'blue', country: 'USA' },
  { type: 'Ka-50', coalition: 'red', country: 'Russia' },
];

let t = 0;
console.log(`Envoi de télémétrie vers ${host}:${port} (Ctrl+C pour arrêter)`);

setInterval(() => {
  t += 1;

  for (const [i, c] of air.entries()) {
    const angle = t / 20 + (i * Math.PI) / 2;
    socket.send(
      JSON.stringify({
        type: 'ownship',
        name: c.name,
        unitType: c.unitType,
        coalition: c.coalition,
        country: c.coalition === 'blue' ? 'USA' : 'Russia',
        lat: center.lat + radius * Math.sin(angle) * 0.4,
        lng: center.lng + radius * Math.cos(angle),
        alt: 3000 + 1500 * Math.sin(angle),
        heading: ((angle * 180) / Math.PI + 360) % 360,
        modelTime: t,
      }),
      port,
      host
    );
  }

  if (t % 2 !== 0) return;

  const units = worldTemplates.map((w, i) => {
    const angle = t / 40 + (i * Math.PI) / 5;
    const r = 0.15 + (i / worldTemplates.length) * 0.6;
    return {
      id: String(100 + i),
      type: w.type,
      coalition: w.coalition,
      country: w.country,
      lat: center.lat + r * Math.sin(angle),
      lng: center.lng + r * Math.cos(angle),
      alt: w.type.startsWith('USS') ? 0 : 100 + i * 15,
      heading: ((angle * 180) / Math.PI + 360) % 360,
    };
  });

  socket.send(JSON.stringify({ type: 'world', count: units.length, units }), port, host);
}, 1000);

process.on('SIGINT', () => {
  socket.close();
  process.exit(0);
});
