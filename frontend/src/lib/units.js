/**
 * Live session store: holds the state pushed by the backend over SSE, plus the
 * theatre selection shared by the airfields tab.
 */
import { writable, get } from 'svelte/store';
import { events, players, chat, mission, sessionRev } from './session.js';

/** All units as received from the backend. */
export const units = writable([]);
export const summary = writable({ byCategory: {}, byCoalition: {} });
export const connected = writable(false);
export const lastUpdate = writable(null);

/**
 * True when DCS has stopped sending telemetry, which happens when the
 * simulation is paused: the export script is only called while time advances.
 */
export const paused = writable(false);

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

      if (msg.type !== 'session') return;
      if (msg.events) events.set(msg.events);
      if (msg.players) players.set(msg.players);
      if (msg.chat) chat.set(msg.chat);
      mission.set(msg.mission ?? null);
      // A paused simulator sends nothing at all; say so rather than looking
      // like a broken app.
      paused.set(Boolean(msg.paused));
      sessionRev.update((n) => n + 1);
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
    if (!storedTheatre() && meta.default && meta.theatres.some((t) => t.id === meta.default)) {
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
