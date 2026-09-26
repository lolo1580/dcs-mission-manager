/**
 * Live session store: game events, players, chat and current mission, fed by
 * the backend over SSE. Mirrors the shape of lib/units.js.
 */
import { writable, derived, get } from 'svelte/store';

export const events = writable([]);
export const players = writable([]);
export const chat = writable([]);
export const mission = writable(null);

/** Bumped whenever any session data changes, to drive "new data" indicators. */
export const sessionRev = writable(0);

export const EVENT_FILTERS = [
  { id: 'all', label: 'Tous' },
  { id: 'kill', label: 'Kills' },
  { id: 'friendly_fire', label: 'Friendly fire' },
  { id: 'crash', label: 'Crashes' },
  { id: 'eject', label: 'Éjections' },
  { id: 'takeoff', label: 'Décollages' },
  { id: 'landing', label: 'Atterrissages' },
  { id: 'pilot_death', label: 'Morts' },
  { id: 'change_slot', label: 'Slots' },
  { id: 'connect', label: 'Connexions' },
  { id: 'disconnect', label: 'Déconnexions' },
];

export const eventFilter = writable('all');

/** Chat messages with a coarse colouring hint based on the sender. */
export const visibleEvents = derived([events, eventFilter], ([$events, $filter]) =>
  $filter === 'all' ? $events : $events.filter((e) => e.event === $filter)
);

/** Event counters, used to show badges on the filter chips. */
export const eventCounts = derived(events, ($events) => {
  const counts = {};
  for (const e of $events) counts[e.event] = (counts[e.event] ?? 0) + 1;
  return counts;
});

export const COALITION_COLORS = {
  red: '#ff4d4d',
  blue: '#3d7dff',
  spectator: '#9aa4b2',
};

export const SIDE_LABELS = {
  0: 'Spectateur',
  1: 'Rouge',
  2: 'Bleu',
};

export function sideKey(side) {
  return side === 1 ? 'red' : side === 2 ? 'blue' : 'spectator';
}
