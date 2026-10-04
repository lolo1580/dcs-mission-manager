/**
 * Career store: the player's own logbook, read from DCS
 * (MissionEditor/logbook.lua) — career totals and a per-airframe breakdown.
 */
import { writable, derived } from 'svelte/store';
import { tNow } from './i18n.js';

/** @type {import('svelte/store').Writable<Array>} */
export const careerPlayers = writable([]);
export const careerCurrent = writable('');
export const careerError = writable('');
export const careerLoading = writable(false);

/** Which profile is shown. Defaults to the first returned. */
export const careerIndex = writable(0);

export async function loadCareer() {
  careerLoading.set(true);
  careerError.set('');
  try {
    const res = await fetch('/api/career');
    if (!res.ok) throw new Error(`${res.status}`);
    const body = await res.json();
    careerPlayers.set(body.players ?? []);
    careerCurrent.set(body.currentPlayer ?? '');
  } catch (e) {
    careerError.set(tNow('error.career', { detail: e.message }));
  } finally {
    careerLoading.set(false);
  }
}

/** The profile currently displayed, or null. */
export const careerPlayer = derived(
  [careerPlayers, careerIndex],
  ([$players, $i]) => $players[$i] ?? $players[0] ?? null
);

/** Total flight hours across all the player's airframes. */
export const careerTotalHours = derived(careerPlayer, ($p) => $p?.totalFlightHours ?? 0);

/** Formats flight hours as "420.8 h" or "1 234 h" for large values. */
export function fmtHours(h) {
  if (typeof h !== 'number' || !Number.isFinite(h)) return '—';
  if (h >= 100) return `${Math.round(h).toLocaleString()} h`;
  return `${h.toFixed(1)} h`;
}

/** A short, readable label for a DCS aircraft type. */
export function aircraftLabel(type) {
  return (type || '').replace(/_/g, ' ');
}
