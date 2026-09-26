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
/** Where the airfield data comes from: "dcs" or "embedded". */
export const aerodromeSource = writable('');
/** Settlements of the active theatre (from DCS data), and whether to draw them. */
export const towns = writable([]);
export const showTowns = writable(false);
/** Aeronautical charts of the selected airfield, and the one being viewed. */
export const aerodromeCharts = writable([]);
export const chartsError = writable('');
export const viewingChart = writable(null);

/** Loads the charts available for one airfield, matched by ICAO and name. */
export async function loadAerodromeCharts(a) {
  chartsError.set('');
  aerodromeCharts.set([]);
  if (!a) return;
  try {
    const q = new URLSearchParams();
    if (a.icaoCode) q.set('icao', a.icaoCode);
    else if (a.id) q.set('icao', a.id);
    if (a.name) q.set('name', a.name);
    const res = await fetch(`/api/charts?${q}`);
    if (!res.ok) throw new Error(`${res.status}`);
    const body = await res.json();
    aerodromeCharts.set(body.charts ?? []);
  } catch (e) {
    chartsError.set(tNow('error.charts', { detail: e.message }));
  }
}

/** URL of a chart image, encoding each path segment (charts live in subfolders). */
export function chartURL(chart) {
  const rel = chart?.path || chart?.name || '';
  return '/api/charts/file/' + rel.split('/').map(encodeURIComponent).join('/');
}
/**
 * Airfield selected on the map. Set from MapView, consumed by UnitDetails (which
 * shares the map's bottom-right corner with the unit card).
 */
export const selectedAerodrome = writable(null);

export function selectAerodrome(a) {
  selectedAerodrome.set(a ?? null);
  loadAerodromeCharts(a);
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
        (a.icaoCode || '').toLowerCase().includes(q) ||
        (a.tacan || '').toLowerCase().includes(q) ||
        (a.vor || '').toLowerCase().includes(q) ||
        (a.rsbn || '').toLowerCase().includes(q)
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
    aerodromeSource.set(body.source ?? '');
    loadTowns(th);
  } catch (e) {
    aerodromeError.set(tNow('error.aerodromes', { detail: e.message }));
  }
}

/** Loads the settlements of a theatre. Empty when DCS data is unavailable. */
export async function loadTowns(theatreId) {
  if (!theatreId) {
    towns.set([]);
    return;
  }
  try {
    const res = await fetch(`/api/towns?theatre=${encodeURIComponent(theatreId)}`);
    if (!res.ok) throw new Error(`${res.status}`);
    const body = await res.json();
    towns.set(body.towns ?? []);
  } catch {
    // Towns are a bonus layer: never surface an error for them.
    towns.set([]);
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
