/**
 * Statistics store: fetches the stats modules for a given scope.
 *
 * Scope is "career" (all missions) or "mission" (one mission).
 */
import { writable, derived } from 'svelte/store';
import { tNow } from './i18n.js';

export const scopeMode = writable('career');

export const statsOverview = writable(null);
export const statsPilots = writable([]);
export const statsWeapons = writable([]);
export const statsEngines = writable([]);
export const statsNetwork = writable([]);
export const statsError = writable('');
export const statsLoading = writable(false);

/** Statistics sub-views. Labels are resolved through i18n at render time. */
export const STAT_TABS = [
  { id: 'pilots', key: 'stats.pilots' },
  { id: 'weapons', key: 'stats.weapons' },
  { id: 'engines', key: 'stats.engines' },
  { id: 'balance', key: 'stats.balance' },
  { id: 'network', key: 'stats.network' },
];

export const statTab = writable('pilots');

/** Query string for the current scope. */
function scopeQuery() {
  let mode;
  scopeMode.subscribe((m) => (mode = m))();
  return mode === 'mission' ? 'scope=mission' : 'scope=career';
}

async function getJSON(path) {
  const res = await fetch(`${path}?${scopeQuery()}`);
  if (!res.ok) throw new Error(`${res.status}`);
  return res.json();
}

export async function loadStats() {
  statsLoading.set(true);
  statsError.set('');
  try {
    const [overview, pilots, weapons, engines, network] = await Promise.all([
      getJSON('/api/stats/overview'),
      getJSON('/api/stats/pilots'),
      getJSON('/api/stats/weapons'),
      getJSON('/api/stats/engines'),
      getJSON('/api/stats/network'),
    ]);
    statsOverview.set(overview);
    statsPilots.set(pilots.pilots ?? []);
    statsWeapons.set(weapons.weapons ?? []);
    statsEngines.set(engines.engines ?? []);
    statsNetwork.set(network.network ?? []);
  } catch (e) {
    statsError.set(tNow('error.stats', { detail: e.message }));
  } finally {
    statsLoading.set(false);
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
