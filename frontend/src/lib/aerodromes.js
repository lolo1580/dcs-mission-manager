/**
 * Aerodrome store: reference airfields (coordinates, radio frequencies, charts).
 */
import { writable, derived, get } from 'svelte/store';
import { tNow } from './i18n.js';
import { units, theatre } from './units.js';

export const aerodromes = writable([]);
export const aerodromeError = writable('');
export const search = writable('');
/** Where the airfield data comes from: "dcs" or "embedded". */
export const aerodromeSource = writable('');
/** Aeronautical charts of the selected airfield, and the one being viewed. */
export const aerodromeCharts = writable([]);
export const chartsError = writable('');
export const viewingChart = writable(null);

// Monotonic id per chart load, so a slow response for a previous airfield cannot
// replace the charts of the one now selected (the classic "A then B, A answers
// last" race).
let chartsSeq = 0;

/** Loads the charts available for one airfield, matched by ICAO and name. */
export async function loadAerodromeCharts(a) {
  const seq = ++chartsSeq;
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
    // A slow response for a previously selected airfield must not replace the
    // charts of the one now shown.
    if (seq !== chartsSeq) return;
    aerodromeCharts.set(body.charts ?? []);
  } catch (e) {
    if (seq !== chartsSeq) return;
    chartsError.set(tNow('error.charts', { detail: e.message }));
  }
}

/** URL of a chart image, encoding each path segment (charts live in subfolders). */
export function chartURL(chart) {
  const rel = chart?.path || chart?.name || '';
  return '/api/charts/file/' + rel.split('/').map(encodeURIComponent).join('/');
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
    const th = get(theatre);
    const res = await fetch(`/api/aerodromes?theatre=${encodeURIComponent(th)}`);
    if (!res.ok) throw new Error(`${res.status}`);
    const body = await res.json();
    aerodromes.set(body.aerodromes ?? []);
    aerodromeSource.set(body.source ?? '');
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
    const th = get(theatre);
    const res = await fetch(`/api/aerodromes?lat=${own.lat}&lng=${own.lng}&theatre=${encodeURIComponent(th)}`);
    if (!res.ok) throw new Error(`${res.status}`);
    const body = await res.json();
    aerodromes.set(body.aerodromes ?? []);
    // The badge must reflect the same source as a plain load: the nearest-field
    // query returns it too, and ignoring it left the badge stale or hidden.
    aerodromeSource.set(body.source ?? '');
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
  if (typeof a?.lat !== 'number' || typeof a?.lng !== 'number') return '—';
  const lat = a.lat >= 0 ? 'N' : 'S';
  const lng = a.lng >= 0 ? 'E' : 'W';
  return `${Math.abs(a.lat).toFixed(4)}°${lat} ${Math.abs(a.lng).toFixed(4)}°${lng}`;
}
