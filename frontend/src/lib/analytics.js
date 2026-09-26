/**
 * Analytics store: heatmap points, flight trails and sortie stats.
 */
import { writable, derived } from 'svelte/store';

export const heatPoints = writable([]);
// Default to traffic: loss clusters are often empty early in a mission.
export const heatSource = writable('positions'); // 'losses' | 'positions'
export const trails = writable({});
export const sortieStats = writable([]);
export const analyticsError = writable('');

export const HEAT_SOURCES = [
  { id: 'positions', label: 'Trafic' },
  { id: 'losses', label: 'Pertes' },
];

/** Total weight of the current heatmap, for the legend. */
export const heatTotal = derived(heatPoints, ($p) =>
  $p.reduce((sum, x) => sum + (x.weight ?? 0), 0)
);

export const maxHeatWeight = derived(heatPoints, ($p) =>
  $p.reduce((max, x) => Math.max(max, x.weight ?? 0), 1)
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
    trails.set(track.trails ?? {});
    sortieStats.set(track.stats ?? []);
  } catch (e) {
    analyticsError.set(`Analyse indisponible : ${e.message}`);
  }
}

export async function reloadHeat() {
  analyticsError.set('');
  try {
    let source;
    heatSource.subscribe((s) => (source = s))();
    const heat = await fetch(`/api/analytics/heatmap?source=${source}`).then((r) => r.json());
    heatPoints.set(heat.points ?? []);
  } catch (e) {
    analyticsError.set(`Heatmap indisponible : ${e.message}`);
  }
}

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
