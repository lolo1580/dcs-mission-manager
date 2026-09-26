/**
 * Unit store: holds the live state pushed by the backend (SSE), plus the
 * client-side filters and selection.
 */
import { writable, derived, get } from 'svelte/store';
import { events, players, chat, mission, sessionRev } from './session.js';

/** All units as received from the backend. @type {import('svelte/store').Writable<Array>} */
export const units = writable([]);
export const summary = writable({ byCategory: {}, byCoalition: {} });
export const connected = writable(false);
export const lastUpdate = writable(null);
/** Fog-of-war policy reported by the backend. */
export const visibility = writable({ mode: 'unknown', label: '—', override: false });

/** @type {import('svelte/store').Writable<string|null>} */
export const selectedId = writable(null);

export const EMPTY_FILTERS = {
  categories: [], // empty = all
  coalitions: [], // empty = all
  search: '',
  ownshipOnly: false,
  showTrails: true,
};

export const filters = writable({ ...EMPTY_FILTERS });

export const CATEGORIES = [
  { id: 'plane', label: 'Avions' },
  { id: 'heli', label: 'Hélicoptères' },
  { id: 'ground', label: 'Sol' },
  { id: 'ship', label: 'Navires' },
  { id: 'structure', label: 'Structures' },
  { id: 'other', label: 'Autres' },
];

export const COALITIONS = [
  { id: 'blue', label: 'Bleu' },
  { id: 'red', label: 'Rouge' },
  { id: 'neutral', label: 'Neutre' },
];

function matches(u, f) {
  if (f.ownshipOnly && !u.ownship) return false;
  if (f.categories.length && !f.categories.includes(u.category)) return false;
  const co = u.coalition || 'neutral';
  if (f.coalitions.length && !f.coalitions.includes(co)) return false;
  if (f.search) {
    const hay = `${u.type} ${u.label || ''} ${u.country || ''}`.toLowerCase();
    if (!hay.includes(f.search.toLowerCase())) return false;
  }
  return true;
}

/** Units passing the current filters. */
export const visibleUnits = derived([units, filters], ([$units, $filters]) =>
  $units.filter((u) => matches(u, $filters))
);

/** The currently selected unit, if still present. */
export const selectedUnit = derived([units, selectedId], ([$units, $id]) =>
  $id ? $units.find((u) => u.id === $id) || null : null
);

export function resetFilters() {
  filters.set({ ...EMPTY_FILTERS });
}

/**
 * Open the SSE stream. Returns a cleanup function.
 */
export function connect() {
  const source = new EventSource('/api/events');

  source.onopen = () => connected.set(true);
  source.onerror = () => connected.set(false);
  source.onmessage = (e) => {
    try {
      const msg = JSON.parse(e.data);

      if (msg.type === 'session') {
        if (msg.events) events.set(msg.events);
        if (msg.players) players.set(msg.players);
        if (msg.chat) chat.set(msg.chat);
        mission.set(msg.mission ?? null);
        sessionRev.update((n) => n + 1);
        return;
      }

      if (msg.type !== 'state') return;
      units.set(msg.units ?? []);
      summary.set(msg.summary ?? { byCategory: {}, byCoalition: {} });
      if (msg.visibility) visibility.set(msg.visibility);
      lastUpdate.set(new Date());

      // Drop the selection if the unit disappeared.
      const id = get(selectedId);
      if (id && !(msg.units ?? []).some((u) => u.id === id)) {
        selectedId.set(null);
      }
    } catch {
      /* ignore malformed frames */
    }
  };

  return () => source.close();
}

export async function fetchTheatres() {
  const res = await fetch('/api/theatres');
  if (!res.ok) throw new Error(`theatres: ${res.status}`);
  return res.json();
}

/** Selected basemap id, persisted across reloads. */
export const basemapId = writable(
  (typeof localStorage !== 'undefined' && localStorage.getItem('dcsmm.basemap')) || 'satellite'
);

/** Basemaps available from the backend (filled once the map is mounted). */
export const basemaps = writable([]);

basemapId.subscribe((id) => {
  if (typeof localStorage !== 'undefined') localStorage.setItem('dcsmm.basemap', id);
});
