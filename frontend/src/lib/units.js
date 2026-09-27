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

/**
 * True when DCS has stopped sending telemetry, which happens when the
 * simulation is paused: the export script is only called while time advances.
 */
export const paused = writable(false);

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

/**
 * Filter definitions. Labels are resolved through i18n at render time
 * (`$t('category.' + id)`, `$t('coalition.' + id)`), so only the ids live here.
 */
export const CATEGORIES = [
  { id: 'plane' },
  { id: 'heli' },
  { id: 'ground' },
  { id: 'ship' },
  { id: 'structure' },
  { id: 'other' },
];

export const COALITIONS = [{ id: 'blue' }, { id: 'red' }, { id: 'neutral' }];

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
        // A paused simulator sends nothing at all; say so rather than looking
        // like a broken app.
        paused.set(Boolean(msg.paused));
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
  const meta = await res.json();
  // Cache the theatre list (with its geographic bounds) so the map can frame
  // itself and outline the DCS map extent without another round trip.
  if (Array.isArray(meta.theatres)) {
    theatres.set(meta.theatres);
    validateTheatrePreference(meta.theatres);
  }
  return meta;
}

/**
 * Corrects a stored theatre that no longer exists. An earlier release used
 * "Marianas" and "Sinai" where DCS says "MarianaIslands" and "SinaiMap"; anyone
 * who saved one of those would be stuck on an empty theatre, with a selector
 * showing no matching option.
 */
function validateTheatrePreference(list) {
  if (!list.length) return;
  const stored = get(theatre);
  if (!list.some((t) => t.id === stored)) {
    const fallback = list.find((t) => t.id === 'Caucasus') ?? list[0];
    theatre.set(fallback.id);
  }
}

/** Known DCS theatres, as returned by /api/theatres. */
export const theatres = writable([]);

/** Active theatre id. Persisted, because a player flies the same map for weeks. */
export const theatre = writable(
  (typeof localStorage !== 'undefined' && localStorage.getItem('dcsmm.theatre')) || 'Caucasus'
);

theatre.subscribe((id) => {
  if (typeof localStorage !== 'undefined') localStorage.setItem('dcsmm.theatre', id);
});

export function setTheatre(id) {
  if (id) theatre.set(id);
}

/** Bounds of the active theatre, or null when it is not known yet. */
export const theatreBounds = derived([theatres, theatre], ([$list, $id]) => {
  const t = $list.find((x) => x.id === $id);
  return t?.bounds ?? null;
});

/** Whether the active theatre has authentic DCS map tiles available. */
export const theatreHasTiles = derived([theatres, theatre], ([$list, $id]) =>
  Boolean($list.find((x) => x.id === $id)?.tiles)
);

/** When true, the DCS map extent is outlined on the map. */
export const showBounds = writable(false);

/** Selected basemap id, persisted across reloads. */
export const basemapId = writable(
  (typeof localStorage !== 'undefined' && localStorage.getItem('dcsmm.basemap')) || 'aero'
);

/** Basemaps available from the backend (filled once the map is mounted). */
export const basemaps = writable([]);

basemapId.subscribe((id) => {
  if (typeof localStorage !== 'undefined') localStorage.setItem('dcsmm.basemap', id);
});
