/**
 * Live store fed by the backend over SSE: unit state (for telemetry-derived
 * views), connection/pause state and the current mission, plus the theatre
 * selection shared by the airfields tab.
 */
import { writable, get } from 'svelte/store';
import { pushPanelEvent, biosState } from './panels.js';
import { pushMappingEvent } from './mappings.js';
import { pushDebugLine } from './debug.js';

/** All units as received from the backend. */
export const units = writable([]);
export const summary = writable({ byCategory: {}, byCoalition: {} });
export const connected = writable(false);
export const lastUpdate = writable(null);

/** The current mission, shown in the header. */
export const mission = writable(null);

/**
 * Waiting for the first DCS export packet, active, or interrupted after data
 * previously arrived. A silent feed is not proof that the simulation is paused.
 */
export const feedStatus = writable('waiting');

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

      if (msg.type === 'state') {
        units.set(msg.units ?? []);
        summary.set(msg.summary ?? { byCategory: {}, byCoalition: {} });
        lastUpdate.set(new Date());
        return;
      }

      // Cockpit hardware arrives on its own frames, pushed as it happens.
      if (msg.type === 'panel') {
        pushPanelEvent(msg.panel);
        return;
      }
      if (msg.type === 'dcsbios' && msg.state) {
        biosState.set(msg.state);
        return;
      }

      // The live mapping test: what a panel input resolves to.
      if (msg.type === 'mapping') {
        pushMappingEvent(msg);
        return;
      }

      // A debug line, when debug logging is on.
      if (msg.type === 'log' && msg.line) {
        pushDebugLine(msg.line);
        return;
      }

      if (msg.type !== 'session') return;
      // The Session tab was removed; the mission and export state remain.
      mission.set(msg.mission ?? null);
      feedStatus.set(msg.feedStopped ? 'interrupted' : msg.feedSeen ? 'active' : 'waiting');
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
  if (Array.isArray(meta.theatres)) {
    theatres.set(meta.theatres);
    // With no stored preference, start on the backend's default theatre.
    if (!hadStoredTheatre && meta.default && meta.theatres.some((t) => t.id === meta.default)) {
      theatre.set(meta.default);
    }
    validateTheatrePreference(meta.theatres);
  }
  return meta;
}

/** The theatre id saved in localStorage, or null. */
function storedTheatre() {
  return typeof localStorage !== 'undefined' ? localStorage.getItem('dcsmanager.theatre') : null;
}

// Captured before the `theatre` store's subscribe writes its default below, so
// "the user never chose a theatre" survives to fetchTheatres(). Checking
// storedTheatre() later always finds the default that was just persisted, and
// the backend default would then never be applied.
const hadStoredTheatre = storedTheatre() !== null;

/**
 * Corrects a stored theatre that is no longer in the list — a map the player
 * uninstalled, say. Without this the selector would sit on an id with no option
 * and the tab would look empty.
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
  (typeof localStorage !== 'undefined' && localStorage.getItem('dcsmanager.theatre')) || 'Caucasus'
);

theatre.subscribe((id) => {
  if (typeof localStorage !== 'undefined') localStorage.setItem('dcsmanager.theatre', id);
});

export function setTheatre(id) {
  if (id) theatre.set(id);
}
