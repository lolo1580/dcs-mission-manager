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
  const req = ++statsReq;
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
