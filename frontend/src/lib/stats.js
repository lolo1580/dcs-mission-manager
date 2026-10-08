/**
 * Statistics store: fetches the stats modules for a given scope.
 *
 * Scope is "career" (all missions) or "mission" (one mission).
 */
import { writable, derived, get } from 'svelte/store';
import { tNow } from './i18n.js';

export const scopeMode = writable('career');
export const selectedMissionID = writable(0);
export const statsMissions = writable([]);
export const statsMissionsError = writable('');
export const statsTrend = writable([]);
export const trendPilotUCID = writable('');
export const trendLoading = writable(false);
export const trendError = writable('');

export const statsOverview = writable(null);
export const statsPilots = writable([]);
export const statsWeapons = writable([]);
export const statsEngines = writable([]);
export const statsNetwork = writable([]);
export const statsError = writable('');
export const statsLoading = writable(false);
/** False when the backend serves statistics without persistence (no database). */
export const statsEnabled = writable(true);

/** Statistics sub-views. Labels are resolved through i18n at render time. */
export const STAT_TABS = [
  { id: 'pilots', key: 'stats.pilots' },
  { id: 'weapons', key: 'stats.weapons' },
  { id: 'engines', key: 'stats.engines' },
  { id: 'balance', key: 'stats.balance' },
  { id: 'network', key: 'stats.network' },
];

export const statTab = writable('pilots');

// Monotonic counter identifying the newest loadStats call, so a slower response
// for a scope the user already left cannot overwrite the current one.
let statsReq = 0;

/** Query string for the current scope. */
function scopeQuery() {
  if (get(scopeMode) !== 'mission') return 'scope=career';
  return `scope=mission&missionId=${get(selectedMissionID)}`;
}

async function getJSON(path, query) {
  const res = await fetch(`${path}?${query}`);
  if (!res.ok) throw new Error(`${res.status}`);
  return res.json();
}

export async function loadStats() {
  const req = ++statsReq;
  const query = scopeQuery();
  statsLoading.set(true);
  statsError.set('');
  try {
    const [overview, pilots, weapons, engines, network] = await Promise.all([
      getJSON('/api/stats/overview', query),
      getJSON('/api/stats/pilots', query),
      getJSON('/api/stats/weapons', query),
      getJSON('/api/stats/engines', query),
      getJSON('/api/stats/network', query),
    ]);
    if (req !== statsReq) return;
    // With persistence off every endpoint answers {enabled:false}; say so
    // instead of showing empty tables as if there were no data.
    if (overview?.enabled === false) {
      statsEnabled.set(false);
      statsOverview.set(null);
      statsPilots.set([]);
      statsWeapons.set([]);
      statsEngines.set([]);
      statsNetwork.set([]);
      return;
    }
    statsEnabled.set(true);
    statsOverview.set(overview);
    statsPilots.set(pilots.pilots ?? []);
    statsWeapons.set(weapons.weapons ?? []);
    statsEngines.set(engines.engines ?? []);
    statsNetwork.set(network.network ?? []);
  } catch (e) {
    if (req !== statsReq) return;
    statsError.set(tNow('error.stats', { detail: e.message }));
  } finally {
    if (req === statsReq) statsLoading.set(false);
  }
}

export async function loadStatsMissions() {
  statsMissionsError.set('');
  try {
    const response = await fetch('/api/stats/missions');
    if (!response.ok) throw new Error(`${response.status}`);
    const body = await response.json();
    const missions = body.missions ?? [];
    statsMissions.set(missions);
    const selected = get(selectedMissionID);
    if (!missions.some((m) => m.id === selected)) selectedMissionID.set(missions[0]?.id ?? 0);
  } catch (error) {
    statsMissionsError.set(tNow('error.stats', { detail: error.message }));
  }
}

let trendReq = 0;
export async function loadTrend() {
  const req = ++trendReq;
  trendLoading.set(true);
  trendError.set('');
  try {
    const query = new URLSearchParams({ ucid: get(trendPilotUCID) });
    const response = await fetch(`/api/stats/trend?${query}`);
    if (!response.ok) throw new Error(`${response.status}`);
    const body = await response.json();
    if (req === trendReq) statsTrend.set(body.missions ?? []);
  } catch (error) {
    if (req === trendReq) trendError.set(tNow('error.stats', { detail: error.message }));
  } finally {
    if (req === trendReq) trendLoading.set(false);
  }
}

/** Weapons sorted by kill count, for the weapons view. */
export const topWeapons = derived(statsWeapons, ($w) => $w.filter((x) => x.kills > 0));

/** The engines of the selected category, toggled by the view. */
export const engineCategory = writable('plane');

export const enginesByCategory = derived(
  [statsEngines, engineCategory],
  ([$engines, $cat]) => $engines.filter((e) => e.category === $cat)
);

export function fmtNum(v, digits = 1) {
  if (typeof v !== 'number') return '—';
  return v.toLocaleString(undefined, { maximumFractionDigits: digits });
}
