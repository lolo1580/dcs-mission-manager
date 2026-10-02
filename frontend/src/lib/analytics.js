/**
 * Analytics store: heatmap points, flight trails and sortie stats.
 */
import { writable, derived } from 'svelte/store';
import { tNow } from './i18n.js';

export const heatPoints = writable([]);
/** Aggregation grid of the heatmap, in degrees (the plot draws cells that size). */
export const heatGrid = writable(0.05);
// Default to traffic: loss clusters are often empty early in a mission.
export const heatSource = writable('positions'); // 'losses' | 'positions'
export const trails = writable({});
export const sortieStats = writable([]);
export const analyticsError = writable('');

/** Whether the plot overlays are drawn. Persisted, as they are a viewing choice. */
function storedFlag(key, def) {
  if (typeof localStorage === 'undefined') return def;
  const v = localStorage.getItem(key);
  return v === null ? def : v === '1';
}
export const showHeat = writable(storedFlag('dcsmanager.an.showHeat', true));
export const showTrails = writable(storedFlag('dcsmanager.an.showTrails', true));
for (const [store, key] of [
  [showHeat, 'dcsmanager.an.showHeat'],
  [showTrails, 'dcsmanager.an.showTrails'],
]) {
  store.subscribe((on) => {
    if (typeof localStorage !== 'undefined') localStorage.setItem(key, on ? '1' : '0');
  });
}

/** Total weight of the current heatmap, for the legend. */
export const heatTotal = derived(heatPoints, ($p) =>
  $p.reduce((sum, x) => sum + (x.weight ?? 0), 0)
);

export async function loadAnalytics() {
  analyticsError.set('');
  try {
    let source;
    heatSource.subscribe((s) => (source = s))();
    const [heat, track] = await Promise.all([
      fetch(`/api/analytics/heatmap?source=${source}`).then((r) => r.json()),
      fetch('/api/analytics/tracks').then((r) => r.json()),
    ]);
    heatPoints.set(heat.points ?? []);
    if (typeof heat.grid === 'number' && heat.grid > 0) heatGrid.set(heat.grid);
    trails.set(track.trails ?? {});
    sortieStats.set(track.stats ?? []);
  } catch (e) {
    analyticsError.set(tNow('error.analytics', { detail: e.message }));
  }
}

export async function reloadHeat() {
  analyticsError.set('');
  try {
    let source;
    heatSource.subscribe((s) => (source = s))();
    const heat = await fetch(`/api/analytics/heatmap?source=${source}`).then((r) => r.json());
    heatPoints.set(heat.points ?? []);
    if (typeof heat.grid === 'number' && heat.grid > 0) heatGrid.set(heat.grid);
  } catch (e) {
    analyticsError.set(tNow('error.heatmap', { detail: e.message }));
  }
}

/** Longest distance among the sorties, so a table bar can be scaled to it. */
export const maxSortieKm = derived(sortieStats, ($s) =>
  $s.reduce((m, x) => Math.max(m, x.distanceKm ?? 0), 0) || 1
);

export function fmtKm(km) {
  if (typeof km !== 'number') return '—';
  return `${km.toLocaleString(undefined, { maximumFractionDigits: 1 })} km`;
}

export function fmtSpeed(mps) {
  if (typeof mps !== 'number' || mps === 0) return '—';
  // m/s to km/h
  return `${Math.round(mps * 3.6)} km/h`;
}

export function fmtDuration(sec) {
  if (typeof sec !== 'number') return '—';
  const m = Math.floor(sec / 60);
  const s = Math.floor(sec % 60);
  return `${m}:${String(s).padStart(2, '0')}`;
}
