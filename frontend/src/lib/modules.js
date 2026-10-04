/**
 * DCS installation store: the module inventory read from DCS itself
 * (MissionEditor/modules.lua) — terrains, aircraft, campaigns, tech packs.
 */
import { writable, derived } from 'svelte/store';
import { tNow } from './i18n.js';

/** @type {import('svelte/store').Writable<Array>} */
export const modules = writable([]);
export const moduleTotals = writable({ count: 0, owned: 0, installed: 0, total: 0 });
export const modulesError = writable('');
export const modulesLoading = writable(false);

/** Free-text filter on the module title, developer or id. */
export const moduleSearch = writable('');

/** When true, only the modules the player owns (bought) are shown. */
export const ownedOnly = writable(false);

/** When true, only the modules actually present on disk are shown. */
export const installedOnly = writable(false);

/** Category filter: '' (all) or one of terrains/moduls/campaigns/bundles. */
export const moduleCategory = writable('');

export async function loadModules() {
  modulesLoading.set(true);
  modulesError.set('');
  try {
    const res = await fetch('/api/modules');
    if (!res.ok) throw new Error(`${res.status}`);
    const body = await res.json();
    modules.set(body.modules ?? []);
    moduleTotals.set({
      count: body.count ?? 0,
      owned: body.owned ?? 0,
      installed: body.installed ?? 0,
      total: body.total ?? 0,
    });
  } catch (e) {
    modulesError.set(tNow('error.modules', { detail: e.message }));
  } finally {
    modulesLoading.set(false);
  }
}

/** Categories present in the inventory, in a stable order. */
export const moduleCategories = derived(modules, ($m) => {
  const order = ['terrains', 'moduls', 'campaigns', 'bundles'];
  const seen = new Set($m.map((x) => x.category));
  return order.filter((c) => seen.has(c));
});

/** Modules passing the search, ownership, install and category filters. */
export const visibleModules = derived(
  [modules, moduleSearch, ownedOnly, installedOnly, moduleCategory],
  ([$m, $q, $owned, $installed, $cat]) => {
    const query = $q.trim().toLowerCase();
    return $m.filter((x) => {
      if ($owned && !x.owned) return false;
      // "Installed" is only meaningful where it is known; an unknown state is
      // never treated as installed.
      if ($installed && !(x.installKnown && x.installed)) return false;
      if ($cat && x.category !== $cat) return false;
      if (!query) return true;
      return `${x.title} ${x.developer ?? ''} ${x.id ?? ''} ${x.type ?? ''}`
        .toLowerCase()
        .includes(query);
    });
  }
);
