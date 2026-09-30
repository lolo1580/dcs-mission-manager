/**
 * Projection-free plotting helpers.
 *
 * The manager has no map any more, so the analytical views plot raw latitude and
 * longitude into a plain pixel box. Longitude is scaled by cos(latitude) so the
 * picture keeps the right proportions instead of being stretched horizontally —
 * the difference is large at high latitudes (Kola, The Channel).
 */

/**
 * Bounding box of the points that carry a numeric position.
 * Returns null when there is nothing to plot.
 *
 * @param {Array<{lat?:number,lng?:number}>} points
 */
export function bounds(points) {
  let minLat = Infinity;
  let maxLat = -Infinity;
  let minLng = Infinity;
  let maxLng = -Infinity;
  let n = 0;
  for (const p of points ?? []) {
    if (typeof p?.lat !== 'number' || typeof p?.lng !== 'number') continue;
    if (!Number.isFinite(p.lat) || !Number.isFinite(p.lng)) continue;
    if (p.lat < minLat) minLat = p.lat;
    if (p.lat > maxLat) maxLat = p.lat;
    if (p.lng < minLng) minLng = p.lng;
    if (p.lng > maxLng) maxLng = p.lng;
    n++;
  }
  if (n === 0) return null;
  return { minLat, maxLat, minLng, maxLng };
}

/**
 * Builds a projector mapping lat/lng to pixels inside a `w`×`h` box, with equal
 * aspect: one degree of longitude is shortened to `cos(latitude)` of a degree of
 * latitude, and the whole shape is centred in the box.
 *
 * @param {{minLat:number,maxLat:number,minLng:number,maxLng:number}} b
 * @param {number} w
 * @param {number} h
 * @param {number} pad
 */
export function projector(b, w, h, pad = 30) {
  const midLat = (b.minLat + b.maxLat) / 2;
  // Clamp the cosine: at the poles it collapses the horizontal axis.
  const kx = Math.max(0.1, Math.cos((midLat * Math.PI) / 180));

  let worldW = (b.maxLng - b.minLng) * kx;
  let worldH = b.maxLat - b.minLat;
  // A single point, or a perfectly straight line, would make the scale infinite.
  const MIN = 1e-4;
  if (worldW < MIN) worldW = MIN;
  if (worldH < MIN) worldH = MIN;

  const scale = Math.min((w - 2 * pad) / worldW, (h - 2 * pad) / worldH);
  const offX = (w - worldW * scale) / 2;
  const offY = (h - worldH * scale) / 2;

  return {
    x: (lng) => offX + (lng - b.minLng) * kx * scale,
    // SVG's y axis grows downward while latitude grows north.
    y: (lat) => h - offY - (lat - b.minLat) * scale,
  };
}

/** Formats a latitude as an axis label. */
export function fmtLat(v) {
  return `${Math.abs(v).toFixed(2)}°${v >= 0 ? 'N' : 'S'}`;
}

/** Formats a longitude as an axis label. */
export function fmtLng(v) {
  return `${Math.abs(v).toFixed(2)}°${v >= 0 ? 'E' : 'W'}`;
}

/**
 * Colours for the track overlay, one per unit. Chosen to stay distinct on the
 * dark panel and to avoid the amber used for warnings.
 */
export const TRACK_COLORS = [
  '#58a6ff',
  '#3fb950',
  '#f0b429',
  '#f85149',
  '#bc8cff',
  '#39c5cf',
  '#ff7b72',
  '#7ee787',
  '#d2a8ff',
  '#79c0ff',
];

/** Colour of a track, cycling through the palette. */
export function trackColor(i) {
  return TRACK_COLORS[i % TRACK_COLORS.length];
}
