/**
 * Debrief store: fetches stored debriefs and exposes the selected one.
 */
import { writable, derived } from 'svelte/store';
import { tNow } from './i18n.js';

export const debriefList = writable([]);
export const debrief = writable(null);
export const debriefError = writable('');
export const debriefLoading = writable(false);

// Monotonic counter identifying the newest loadDebrief call, used to drop an
// older response that resolves after a newer one.
let debriefReq = 0;

/** Events of the selected debrief, newest last. */
export const debriefEvents = derived(debrief, ($d) => $d?.parsed?.events ?? []);

/** Kills extracted from the selected debrief, for the summary view. */
export const debriefKills = derived(debriefEvents, ($events) =>
  $events.filter((e) => e.type === 'kill' || e.type === 'shot down')
);

export async function loadDebriefList() {
  try {
    const res = await fetch('/api/debriefs');
    if (!res.ok) throw new Error(`${res.status}`);
    const body = await res.json();
    debriefList.set(body.debriefs ?? []);
  } catch (e) {
    debriefError.set(tNow('error.debriefList', { detail: e.message }));
  }
}

export async function loadDebrief(id) {
  // Guard against out-of-order responses: clicking two debriefs quickly can
  // resolve the first after the second, and would otherwise leave the detail
  // pane showing stats that belong to a different, earlier debrief.
  const req = ++debriefReq;
  debriefLoading.set(true);
  debriefError.set('');
  try {
    const res = await fetch(`/api/debriefs/${id}`);
    if (!res.ok) throw new Error(`${res.status}`);
    const body = await res.json();
    if (req !== debriefReq) return;
    debrief.set(body);
  } catch (e) {
    if (req !== debriefReq) return;
    debriefError.set(tNow('error.debrief', { detail: e.message }));
    debrief.set(null);
  } finally {
    if (req === debriefReq) debriefLoading.set(false);
  }
}

/** Format a mission time (seconds) as h:mm:ss. */
export function fmtTime(seconds) {
  if (typeof seconds !== 'number') return '—';
  const s = Math.floor(seconds % 60);
  const m = Math.floor((seconds / 60) % 60);
  const h = Math.floor(seconds / 3600);
  const pad = (n) => String(n).padStart(2, '0');
  return h > 0 ? `${h}:${pad(m)}:${pad(s)}` : `${m}:${pad(s)}`;
}
