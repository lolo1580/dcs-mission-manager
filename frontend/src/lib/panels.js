/**
 * Panels store: the Logitech/Saitek flight panels the manager drives, and the
 * DCS-BIOS link it listens to. Fed by the API and updated live over SSE.
 */
import { writable, derived } from 'svelte/store';
import { tNow } from './i18n.js';

/** @type {import('svelte/store').Writable<Array>} */
export const panels = writable([]);
export const panelsSupported = writable(true);
export const panelsError = writable('');
export const panelsLoading = writable(false);

/** DCS-BIOS link state: { available, connected, aircraft, frames }. */
export const biosState = writable({ available: true, connected: false, aircraft: '', frames: 0 });

/** The recent input events, newest first, for a live monitor. */
export const panelEvents = writable([]);

const MAX_EVENTS = 200;

export async function loadPanels() {
  panelsLoading.set(true);
  panelsError.set('');
  try {
    const [panelsRes, biosRes] = await Promise.all([
      fetch('/api/panels').then((r) => r.json()),
      fetch('/api/dcsbios').then((r) => r.json()),
    ]);
    panels.set(panelsRes.devices ?? []);
    panelsSupported.set(panelsRes.supported ?? false);
    if (panelsRes.bios) biosState.set(panelsRes.bios);
    if (biosRes) biosState.set(biosRes);
  } catch (e) {
    panelsError.set(tNow('error.panels', { detail: e.message }));
  } finally {
    panelsLoading.set(false);
  }
}

/** Records a panel event pushed over SSE, keeping the list bounded. */
export function pushPanelEvent(ev) {
  panelEvents.update((list) => [ev, ...list].slice(0, MAX_EVENTS));
}

/** True when at least one panel is connected. */
export const hasPanels = derived(panels, ($p) => $p.length > 0);

/** A short label for a panel model id. */
export function modelLabel(model) {
  switch (model) {
    case 'pz55':
      return 'PZ55 Switch Panel';
    case 'pz70':
      return 'PZ70 Multi Panel';
    default:
      return model || '—';
  }
}

/** Formats a USB vendor/product pair as VID/PID. */
export function fmtIDs(vendorId, productId) {
  const hex = (n) => '0x' + (n ?? 0).toString(16).toUpperCase().padStart(4, '0');
  return `${hex(vendorId)} / ${hex(productId)}`;
}
