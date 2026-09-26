// tools/send-events.mjs
//
// Émetteur d'événements de test : simule ce que DCS enverrait via Hooks/dcsmm.lua
// sur le canal TCP. Permet de valider la Phase 2 (joueurs, événements, chat)
// sans lancer DCS.
//
// Usage :
//   node tools/send-events.mjs [host] [port]
//   node tools/send-events.mjs 127.0.0.1 7779

import net from 'node:net';

const host = process.argv[2] ?? '127.0.0.1';
const port = Number(process.argv[3] ?? 7779);

const socket = net.createConnection({ host, port }, () => {
  console.log(`Connecté à ${host}:${port}`);
  start();
});

socket.on('error', (err) => {
  console.error(`Erreur: ${err.message}`);
  process.exit(1);
});

function send(obj) {
  socket.write(JSON.stringify(obj) + '\n');
}

const pilots = [
  { id: 1, ucid: 'ucid-alpha', name: 'Viper', side: 2, slot: 'F-16C_50' },
  { id: 2, ucid: 'ucid-bravo', name: 'Hornet', side: 2, slot: 'FA-18C_hornet' },
  { id: 3, ucid: 'ucid-charlie', name: 'Flanker', side: 1, slot: 'Su-27' },
  { id: 4, ucid: 'ucid-delta', name: 'Hind', side: 1, slot: 'Mi-24P' },
];

const stats = new Map(
  pilots.map((p) => [
    p.id,
    { score: 0, air: 0, car: 0, ship: 0, landings: 0, ejects: 0, crashes: 0 },
  ])
);

function roster() {
  return pilots.map((p) => {
    const s = stats.get(p.id);
    return {
      id: p.id,
      ucid: p.ucid,
      name: p.name,
      side: p.side,
      slot: p.slot,
      unitType: p.slot,
      ping: 20 + Math.floor(Math.random() * 180),
      crashes: s.crashes,
      killsCar: s.car,
      killsAir: s.air,
      killsShip: s.ship,
      score: s.score,
      landings: s.landings,
      ejects: s.ejects,
      updatedAt: Date.now(),
    };
  });
}

let t = 0;
const EVENT_SEQUENCE = ['takeoff', 'kill', 'landing', 'crash', 'eject', 'kill', 'friendly_fire'];

function start() {
  // Mission start + initial roster.
  send({ type: 'mission', phase: 'start', name: 'Test Phase 2', theatre: 'Caucasus' });
  send({ type: 'players', players: roster() });

  setInterval(() => {
    t += 1;

    // Event every other tick.
    if (t % 2 === 0) {
      const kind = EVENT_SEQUENCE[(t / 2) % EVENT_SEQUENCE.length];
      const actor = pilots[t % pilots.length];
      const victim = pilots[(t + 1) % pilots.length];
      emit(kind, actor, victim);
    }

    // Roster refresh, like the Lua side does every 5 s.
    if (t % 5 === 0) send({ type: 'players', players: roster() });

    // Occasional chat.
    if (t % 7 === 0) {
      send({ type: 'chat', from: pilots[t % pilots.length].name, message: `Message de test #${t}` });
    }
  }, 1000);
}

function emit(kind, actor, victim) {
  const s = stats.get(actor.id);
  let args = [];
  switch (kind) {
    case 'takeoff':
      args = [actor.id, 100 + actor.id, 'Batumi'];
      break;
    case 'landing':
      args = [actor.id, 100 + actor.id, 'Batumi'];
      s.landings += 1;
      s.score += 25;
      break;
    case 'kill':
      args = [actor.id, actor.slot, actor.side, victim.id, victim.slot, victim.side, 'AIM-120C'];
      if (victim.side === 2) s.air += 1;
      else s.car += 1;
      s.score += 100;
      break;
    case 'friendly_fire':
      args = [actor.id, 'AIM-120C', victim.id];
      s.score -= 50;
      break;
    case 'crash':
      args = [actor.id, 100 + actor.id];
      s.crashes += 1;
      break;
    case 'eject':
      args = [actor.id, 100 + actor.id];
      s.ejects += 1;
      break;
  }
  send({ type: 'event', event: kind, args, t });
  console.log(`événement ${kind}: ${actor.name}${victim && kind === 'kill' ? ' → ' + victim.name : ''}`);
}

process.on('SIGINT', () => {
  send({ type: 'mission', phase: 'end', winner: 'blue' });
  socket.end();
  process.exit(0);
});
