/**
 * Aerodrome store: reference airfields (coordinates, radio frequencies, charts).
 */
import { writable, derived, get } from 'svelte/store';
import { tNow } from './i18n.js';
import { units, theatre } from './units.js';

export const aerodromes = writable([]);
export const aerodromeError = writable('');
export const search = writable('');
/** When true, airfield markers are drawn on the map. */
export const showOnMap = writable(false);
/**
 * Airfield selected on the map. Set from MapView, consumed by UnitDetails (which
 * shares the map's bottom-right corner with the unit card).
 */
export const selectedAerodrome = writable(null);

export function selectAerodrome(a) {
  selectedAerodrome.set(a ?? null);
}

export const filteredAerodromes = derived(
  [aerodromes, search],
  ([$list, $search]) => {
    const q = $search.trim().toLowerCase();
    if (!q) return $list;
    return $list.filter(
      (a) =>
        a.name.toLowerCase().includes(q) ||
        a.id.toLowerCase().includes(q) ||
        (a.tacan || '').toLowerCase().includes(q)
    );
  }
);

export async function loadAerodromes() {
  aerodromeError.set('');
  try {
    let th;
    theatre.subscribe((v) => (th = v))();
    const res = await fetch(`/api/aerodromes?theatre=${encodeURIComponent(th)}`);
    if (!res.ok) throw new Error(`${res.status}`);
    const body = await res.json();
    aerodromes.set(body.aerodromes ?? []);
  } catch (e) {
    aerodromeError.set(tNow('error.aerodromes', { detail: e.message }));
  }
}

/**
 * Loads airfields annotated with distance from the ownship's current position,
 * sorted by proximity, so the nearest field is first.
 */
export async function loadNearest() {
  aerodromeError.set('');
  let own;
  units.subscribe((list) => (own = list.find((u) => u.ownship)))();
  if (!own) {
    await loadAerodromes();
    return false;
  }
  try {
    const res = await fetch(`/api/aerodromes?lat=${own.lat}&lng=${own.lng}`);
    if (!res.ok) throw new Error(`${res.status}`);
    const body = await res.json();
    aerodromes.set(body.aerodromes ?? []);
    return true;
  } catch (e) {
    aerodromeError.set(tNow('error.aerodromes', { detail: e.message }));
    return false;
  }
}

export function fmtMHz(v) {
  if (!v) return '—';
  return `${v.toFixed(3)} MHz`;
}

export function fmtCoords(a) {
  const lat = a.lat >= 0 ? 'N' : 'S';
  const lng = a.lng >= 0 ? 'E' : 'W';
  return `${Math.abs(a.lat).toFixed(4)}°${lat} ${Math.abs(a.lng).toFixed(4)}°${lng}`;
}
