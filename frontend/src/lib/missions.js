/**
 * Mission library: the .miz files saved in DCS's Saved Games folder, described
 * by the metadata each one carries (theatre, date, weather, size).
 */
import { writable, derived } from 'svelte/store';
import { tNow } from './i18n.js';

/** @type {import('svelte/store').Writable<Array>} */
export const missions = writable([]);
export const missionTotals = writable({ count: 0, total: 0 });
export const missionsError = writable('');
export const missionsLoading = writable(false);

/** Free-text filter on the mission name or theatre. */
export const missionSearch = writable('');

export async function loadMissions() {
  missionsLoading.set(true);
  missionsError.set('');
  try {
    const res = await fetch('/api/missions');
    if (!res.ok) throw new Error(`${res.status}`);
    const body = await res.json();
    missions.set(body.missions ?? []);
    missionTotals.set({ count: body.count ?? 0, total: body.total ?? 0 });
  } catch (e) {
    missionsError.set(tNow('error.missions', { detail: e.message }));
  } finally {
    missionsLoading.set(false);
  }
}

/** Missions passing the search box. */
export const visibleMissions = derived([missions, missionSearch], ([$m, $q]) => {
  const query = $q.trim().toLowerCase();
  if (!query) return $m;
  return $m.filter((x) =>
    `${x.name} ${x.theatre ?? ''} ${x.weather ?? ''}`.toLowerCase().includes(query)
  );
});

/** Formats a file size as "1.2 MB" or "840 KB". */
export function fmtSize(bytes) {
  if (typeof bytes !== 'number') return '—';
  if (bytes >= 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  return `${Math.round(bytes / 1024)} KB`;
}

/** Formats an in-game start time (seconds since midnight) as HH:MM. */
export function fmtClock(sec) {
  if (typeof sec !== 'number' || sec <= 0) return '—';
  const h = Math.floor(sec / 3600) % 24;
  const m = Math.floor((sec % 3600) / 60);
  return `${String(h).padStart(2, '0')}:${String(m).padStart(2, '0')}`;
}

/** Formats a unix-ms timestamp as a local date. */
export function fmtDate(ms) {
  if (typeof ms !== 'number' || ms <= 0) return '—';
  return new Date(ms).toLocaleString();
}
