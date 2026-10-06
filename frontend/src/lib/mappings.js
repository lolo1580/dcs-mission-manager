/**
 * Mapping store: binds a panel control to a DCS-BIOS command.
 *
 * Sending is off by default and stays off across restarts, so this store exposes
 * the safety switch explicitly rather than folding it into a profile save.
 */
import { writable, derived, get } from 'svelte/store';
import { tNow } from './i18n.js';

/** The bindings of the aircraft currently being edited. */
export const bindings = writable([]);
/** The LED/display output bindings of the aircraft currently being edited. */
export const outputs = writable([]);
/** The PZ70 LCD display bindings of the aircraft currently being edited. */
export const displays = writable([]);
/** Whether commands are actually sent. */
export const sendingEnabled = writable(false);
/** Whether the panel outputs (LEDs, LCD) are driven. Separate from sending. */
export const outputsEnabled = writable(false);
/** Whether the live mapping test is on (panel inputs reach DCS-BIOS anyway). */
export const testMode = writable(false);
export const mappingsError = writable('');
export const mappingsLoading = writable(false);

/** The aircraft the bindings belong to (the active one, normally). */
export const mappingAircraft = writable('');

/** The aircraft names that have a profile to edit. */
export const aircraftList = writable([]);

/** Loads the list of editable aircraft (those with a seeded or saved profile). */
export async function loadAircraft() {
  try {
    const r = await fetch('/api/aircraft');
    if (!r.ok) return;
    const body = await r.json();
    aircraftList.set(body.aircraft ?? []);
  } catch {
    /* best-effort */
  }
}

/** The DCS-BIOS catalogue of that aircraft: the commands a control can drive. */
export const controls = writable([]);
export const controlsAvailable = writable(false);

// A monotonically increasing id per load, so a slow response for a previous
// aircraft cannot overwrite the bindings of the current one (the classic "A then
// B, B answers first" race that mixes two profiles).
let loadSeq = 0;

export async function loadMappings(aircraft) {
  if (!aircraft) return;
  const seq = ++loadSeq;
  mappingsLoading.set(true);
  mappingsError.set('');
  try {
    mappingAircraft.set(aircraft);
    const getJSON = async (url) => {
      const r = await fetch(url);
      if (!r.ok) throw new Error(`${url}: ${r.status}`);
      return r.json();
    };
    const [profileRes, controlsRes] = await Promise.all([
      getJSON(`/api/mappings?aircraft=${encodeURIComponent(aircraft)}`),
      // The whole catalogue: writable controls are the command bindings, read-only
      // ones (LEDs, gauges) are what an LED output binding reads.
      getJSON(`/api/controls?aircraft=${encodeURIComponent(aircraft)}`),
    ]);
    if (seq !== loadSeq) return; // a newer load superseded this one: drop the result
    bindings.set(profileRes.profile?.bindings ?? []);
    outputs.set(profileRes.profile?.outputs ?? []);
    displays.set(profileRes.profile?.displays ?? []);
    sendingEnabled.set(Boolean(profileRes.enabled));
    outputsEnabled.set(Boolean(profileRes.outputs));
    controls.set(controlsRes.controls ?? []);
    controlsAvailable.set(Boolean(controlsRes.available));
  } catch (e) {
    if (seq !== loadSeq) return;
    // Invalidate the stale profile rather than leaving the previous aircraft's
    // bindings on screen under the new aircraft's name.
    bindings.set([]);
    outputs.set([]);
    displays.set([]);
    controls.set([]);
    controlsAvailable.set(false);
    mappingsError.set(tNow('error.mappings', { detail: e.message }));
  } finally {
    if (seq === loadSeq) mappingsLoading.set(false);
  }
}

/** Turns the live mapping test on or off. */
export async function setTestMode(on) {
  mappingsError.set('');
  try {
    const res = await fetch('/api/mappings/test', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ enabled: on }),
    });
    if (!res.ok) {
      const body = await res.json().catch(() => ({}));
      mappingsError.set(body.error ?? `${res.status}`);
      return false;
    }
    const body = await res.json();
    testMode.set(Boolean(body.enabled));
    return true;
  } catch (e) {
    mappingsError.set(tNow('error.mappings', { detail: e.message }));
    return false;
  }
}

/** The commands the last panel inputs produced (the live mapping test log). */
export const mappingEvents = writable([]);
const MAX_MAPPING_EVENTS = 100;

/** Records one mapping result pushed over SSE, newest first, bounded. */
export function pushMappingEvent(ev) {
  mappingEvents.update((list) => [ev, ...list].slice(0, MAX_MAPPING_EVENTS));
}
export async function setSending(on) {
  mappingsError.set('');
  try {
    const res = await fetch('/api/mappings/safety', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ enabled: on }),
    });
    if (!res.ok) {
      const body = await res.json().catch(() => ({}));
      mappingsError.set(body.error ?? `${res.status}`);
      return false;
    }
    const body = await res.json();
    sendingEnabled.set(Boolean(body.enabled));
    return true;
  } catch (e) {
    mappingsError.set(tNow('error.mappings', { detail: e.message }));
    return false;
  }
}

/** Replaces the aircraft's bindings. */
export async function saveMappings(list) {
  return saveProfile({ bindings: list, outputs: get(outputs), displays: get(displays) });
}

/** Turns the panel outputs (LEDs, LCD) on or off. Independent of command sending. */
export async function setOutputs(on) {
  mappingsError.set('');
  try {
    const res = await fetch('/api/mappings/outputs', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ enabled: on }),
    });
    if (!res.ok) {
      const body = await res.json().catch(() => ({}));
      mappingsError.set(body.error ?? `${res.status}`);
      return false;
    }
    const body = await res.json();
    outputsEnabled.set(Boolean(body.enabled));
    return true;
  } catch (e) {
    mappingsError.set(tNow('error.mappings', { detail: e.message }));
    return false;
  }
}

/** Replaces the aircraft's LED output bindings. */
export async function saveOutputs(list) {
  return saveProfile({ bindings: get(bindings), outputs: list, displays: get(displays) });
}

/** Replaces the aircraft's PZ70 LCD display bindings. */
export async function saveDisplays(list) {
  return saveProfile({ bindings: get(bindings), outputs: get(outputs), displays: list });
}

/** Adds an LCD display binding, refusing a duplicate mode+line like the backend. */
export async function addDisplay(binding) {
  const current = get(displays);
  const same = (d) => d.mode === binding.mode && d.line === binding.line;
  if (current.some(same)) {
    mappingsError.set(tNow('error.displayDuplicate'));
    return false;
  }
  return saveDisplays([...current, binding]);
}

/** Removes an LCD display binding by its mode and line. */
export async function removeDisplay(mode, line) {
  const current = get(displays);
  return saveDisplays(current.filter((d) => !(d.mode === mode && d.line === line)));
}

/** The result of the last display preview: { available, raw, value, text, reason }. */
export const displayPreview = writable(null);

/** Resolves a display binding against the live DCS-BIOS memory (no hardware). */
export async function previewDisplay(params) {
  displayPreview.set(null);
  try {
    const r = await fetch('/api/display/preview', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(params),
    });
    displayPreview.set(await r.json());
  } catch {
    displayPreview.set({ available: false, reason: 'unreachable' });
  }
}

/** Writes chosen numbers to every connected PZ70, to check the LCD hardware. */
export async function testDisplay({ upper, lower }) {
  try {
    const r = await fetch('/api/display/test', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ upper, lower }),
    });
    return r.ok;
  } catch {
    return false;
  }
}

/**
 * Writes the whole profile (bindings + outputs + displays) in one request. Every
 * field is sent, so a change to one never depends on the server remembering the
 * others.
 */
async function saveProfile({ bindings: nextBindings, outputs: nextOutputs, displays: nextDisplays }) {
  const aircraft = get(mappingAircraft);
  if (!aircraft) return false;
  mappingsError.set('');
  try {
    const res = await fetch(`/api/mappings?aircraft=${encodeURIComponent(aircraft)}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ bindings: nextBindings, outputs: nextOutputs, displays: nextDisplays }),
    });
    if (!res.ok) {
      const body = await res.json().catch(() => ({}));
      mappingsError.set(body.error ?? `${res.status}`);
      return false;
    }
    const body = await res.json();
    bindings.set(body.profile?.bindings ?? []);
    outputs.set(body.profile?.outputs ?? []);
    displays.set(body.profile?.displays ?? []);
    return true;
  } catch (e) {
    mappingsError.set(tNow('error.mappings', { detail: e.message }));
    return false;
  }
}

/** Adds a binding, refusing a duplicate control like the backend does. */
export async function addBinding(binding) {
  const current = get(bindings);
  if (current.some((b) => b.model === binding.model && b.control === binding.control)) {
    mappingsError.set(tNow('error.mappingDuplicate'));
    return false;
  }
  return saveMappings([...current, binding]);
}

/** Removes a binding by its control key. */
export async function removeBinding(model, control) {
  const current = get(bindings);
  return saveMappings(current.filter((b) => !(b.model === model && b.control === control)));
}

/** Adds an LED output binding, refusing a duplicate target like the backend does. */
export async function addOutput(binding) {
  const current = get(outputs);
  if (current.some((o) => o.model === binding.model && o.target === binding.target)) {
    mappingsError.set(tNow('error.mappingDuplicate'));
    return false;
  }
  return saveOutputs([...current, binding]);
}

/** Removes an LED output binding by its panel and target. */
export async function removeOutput(model, target) {
  const current = get(outputs);
  return saveOutputs(current.filter((o) => !(o.model === model && o.target === target)));
}

/** True when sending is on and at least one binding exists. */
export const mappingsActive = derived(
  [bindings, sendingEnabled],
  ([$b, $on]) => $on && $b.length > 0
);
