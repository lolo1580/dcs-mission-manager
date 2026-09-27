// DCS terrain vectors: the geography that exists *in the game* — roads,
// railroads, rivers, water bodies, borders, urban areas — drawn over the map.
//
// These layers come from DCS's own terrain data, exported as GeoJSON by
// `dcsmm import-vectors`. Showing them is what makes the map match what the
// simulator actually contains, rather than a real-world approximation.
import { writable, get } from 'svelte/store';
import { theatre } from './units.js';

/**
 * Layers available for the active theatre, as returned by /api/vectors.
 * @type {import('svelte/store').Writable<Array<{name:string,file:string,bounds?:number[],size?:number}>>}
 */
export const vectorLayers = writable([]);

/** Names of the layers currently drawn. */
export const shownVectors = writable(new Set());

/** Whether the whole vector overlay is on. */
export const showVectors = writable(false);

/** One entry per layer, with the label shown in the UI.
 *
 * Order matters: the first match wins, and "railroads" contains "roads", so the
 * longer, more specific names come first.
 */
const LAYER_LABELS = [
  ['railroad', 'Railroads'],
  ['waterbed', 'Water'],
  ['rivers', 'Rivers'],
  ['urban', 'Urban areas'],
  ['borders', 'Borders'],
  ['airbase', 'Airfields (area)'],
  ['roads', 'Roads'],
  ['towns', 'Towns'],
];

/**
 * Layers that add something over the DCS basemap.
 *
 * The map's own tiles already draw roads, railroads, rivers, water, urban areas,
 * borders and settlements: enabling those as vectors paints the same thing twice.
 * The airfield footprints are the exception — DCS's raster map does not outline
 * them, so that layer is the one worth offering.
 *
 * The importer and the server still handle every layer, so re-offering one is a
 * one-line change here. They are genuinely useful over a satellite or relief
 * basemap, where nothing is drawn by the tiles.
 */
const USEFUL_LAYERS = ['airbase'];

/** Whether a layer is worth showing in the picker. */
export function isUsefulLayer(name) {
  return USEFUL_LAYERS.some((k) => name.toLowerCase().includes(k));
}

/** Human label for a layer file name. */
export function layerLabel(name) {
  const lower = name.toLowerCase();
  for (const [key, label] of LAYER_LABELS) {
    if (lower.includes(key)) return label;
  }
  return name;
}

/** Order in which layers should be drawn, so water sits under roads. */
export function layerRank(name) {
  const lower = name.toLowerCase();
  const order = ['waterbeds', 'rivers', 'urban', 'borders', 'railroads', 'roads', 'airbases'];
  for (let i = 0; i < order.length; i++) {
    if (lower.includes(order[i])) return i;
  }
  return order.length;
}

/** Fetches the layer list for the active theatre. */
export async function loadVectors() {
  const th = get(theatre);
  if (!th) {
    vectorLayers.set([]);
    return;
  }
  try {
    const res = await fetch(`/api/vectors?theatre=${encodeURIComponent(th)}`);
    if (!res.ok) throw new Error(String(res.status));
    const data = await res.json();
    const layers = (data.layers ?? [])
      .filter((l) => isUsefulLayer(l.name))
      .sort((a, b) => layerRank(a.name) - layerRank(b.name));
    vectorLayers.set(layers);
  } catch {
    vectorLayers.set([]);
  }
}

/** Whether a layer is currently drawn. */
export function isShown(name) {
  return get(shownVectors).has(name);
}

/** Turns one layer on or off. */
export function toggleVector(name) {
  shownVectors.update((s) => {
    const next = new Set(s);
    if (next.has(name)) next.delete(name);
    else next.add(name);
    return next;
  });
}

/** Turns every layer on or off at once. */
export function toggleAllVectors(on) {
  showVectors.set(on);
  if (!on) {
    shownVectors.set(new Set());
    return;
  }
  shownVectors.set(new Set(get(vectorLayers).map((l) => l.name)));
}

/** URL of a layer's GeoJSON. */
export function vectorURL(file) {
  return `/api/vectors/file/${file}`;
}
