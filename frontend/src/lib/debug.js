/**
 * Debug store: the runtime debug switch and the live log it records.
 *
 * Debug logging is off by default. When on, the backend pushes a line for every
 * notable action (HTTP requests, panel inputs and the commands they produce,
 * DCS-BIOS frames, mission and debrief transitions), which is shown here and in
 * the file log. Handy to watch "what is the manager doing?" without a console.
 */
import { writable, derived } from 'svelte/store';

/** Whether debug logging is on. */
export const debugEnabled = writable(false);
/** Whether the backend has a debug logger at all. */
export const debugAvailable = writable(true);
/** The recorded lines, newest first. */
export const debugLines = writable([]);

const MAX_LINES = 500;

/** True when at least one line has been recorded. */
export const hasDebugLines = derived(debugLines, ($l) => $l.length > 0);

/** Loads the current switch state and log. */
export async function loadDebug() {
  try {
    const r = await fetch('/api/debug');
    if (!r.ok) return;
    const body = await r.json();
    debugEnabled.set(Boolean(body.enabled));
    debugAvailable.set(body.available !== false);
    debugLines.set(body.lines ?? []);
  } catch {
    /* the endpoint is best-effort */
  }
}

/** Turns debug logging on or off. */
export async function setDebug(on) {
  try {
    const r = await fetch('/api/debug', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ enabled: on }),
    });
    if (!r.ok) return false;
    const body = await r.json();
    debugEnabled.set(Boolean(body.enabled));
    return true;
  } catch {
    return false;
  }
}

/** Empties the log (in memory only; the file keeps its history). */
export async function clearDebug() {
  try {
    await fetch('/api/log', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ clear: true }),
    });
    debugLines.set([]);
  } catch {
    /* ignore */
  }
}

/** Records a line pushed over SSE, newest first, bounded. */
export function pushDebugLine(line) {
  debugLines.update((list) => [line, ...list].slice(0, MAX_LINES));
}

/** Formats a debug line's level for display. */
export function levelLabel(level) {
  return level || 'info';
}
