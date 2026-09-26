/**
 * Live session store: game events, players, chat and current mission, fed by
 * the backend over SSE. Mirrors the shape of lib/units.js.
 */
import { writable, derived, get } from 'svelte/store';
import { tNow } from './i18n.js';

export const events = writable([]);
export const players = writable([]);
export const chat = writable([]);
export const mission = writable(null);

/** Bumped whenever any session data changes, to drive "new data" indicators. */
export const sessionRev = writable(0);

/**
 * Event filter definitions. Labels are resolved through i18n at render time
 * (`$t('events.all')`, `$t('events.kills')`, …) so only ids live here.
 */
export const EVENT_FILTERS = [
  { id: 'all', key: 'events.all' },
  { id: 'kill', key: 'events.kills' },
  { id: 'friendly_fire', key: 'events.friendlyFire' },
  { id: 'crash', key: 'events.crashes' },
  { id: 'eject', key: 'events.ejections' },
  { id: 'takeoff', key: 'events.takeoffs' },
  { id: 'landing', key: 'events.landings' },
  { id: 'pilot_death', key: 'events.deaths' },
  { id: 'change_slot', key: 'events.slots' },
  { id: 'connect', key: 'events.connections' },
  { id: 'disconnect', key: 'events.disconnections' },
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

/** Maps a DCS side id to a coalition key, for colouring and i18n labels. */
export function sideKey(side) {
  return side === 1 ? 'red' : side === 2 ? 'blue' : 'spectator';
}

/** Translates a DCS side id through the i18n dictionaries. */
export function sideLabel(side) {
  return tNow(`coalition.${sideKey(side)}`);
}
