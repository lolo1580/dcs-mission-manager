/**
 * Debrief store: fetches stored debriefs and exposes the selected one.
 */
import { writable, derived } from 'svelte/store';

export const debriefList = writable([]);
export const debrief = writable(null);
export const debriefError = writable('');
export const debriefLoading = writable(false);

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
    debriefError.set(`Liste indisponible : ${e.message}`);
  }
}

export async function loadDebrief(id) {
  debriefLoading.set(true);
  debriefError.set('');
  try {
    const res = await fetch(`/api/debriefs/${id}`);
    if (!res.ok) throw new Error(`${res.status}`);
    debrief.set(await res.json());
  } catch (e) {
    debriefError.set(`Débrief indisponible : ${e.message}`);
    debrief.set(null);
  } finally {
    debriefLoading.set(false);
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
