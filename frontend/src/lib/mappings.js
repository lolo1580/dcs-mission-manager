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
/** Whether commands are actually sent. */
export const sendingEnabled = writable(false);
export const mappingsError = writable('');
export const mappingsLoading = writable(false);

/** The aircraft the bindings belong to (the active one, normally). */
export const mappingAircraft = writable('');

/** The DCS-BIOS catalogue of that aircraft: the commands a control can drive. */
export const controls = writable([]);
export const controlsAvailable = writable(false);

export async function loadMappings(aircraft) {
  if (!aircraft) return;
  mappingsLoading.set(true);
  mappingsError.set('');
  try {
    mappingAircraft.set(aircraft);
    const [profileRes, controlsRes] = await Promise.all([
      fetch(`/api/mappings?aircraft=${encodeURIComponent(aircraft)}`).then((r) => r.json()),
      fetch(`/api/controls?aircraft=${encodeURIComponent(aircraft)}&writable=1`).then((r) => r.json()),
    ]);
    bindings.set(profileRes.profile?.bindings ?? []);
    sendingEnabled.set(Boolean(profileRes.enabled));
    controls.set(controlsRes.controls ?? []);
    controlsAvailable.set(Boolean(controlsRes.available));
  } catch (e) {
    mappingsError.set(tNow('error.mappings', { detail: e.message }));
  } finally {
    mappingsLoading.set(false);
  }
}

/** Turns command sending on or off. Refused server-side when it cannot work. */
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
  const aircraft = get(mappingAircraft);
  if (!aircraft) return false;
  mappingsError.set('');
  try {
    const res = await fetch(`/api/mappings?aircraft=${encodeURIComponent(aircraft)}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ bindings: list }),
    });
    if (!res.ok) {
      const body = await res.json().catch(() => ({}));
      mappingsError.set(body.error ?? `${res.status}`);
      return false;
    }
    const body = await res.json();
    bindings.set(body.profile?.bindings ?? []);
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

/** True when sending is on and at least one binding exists. */
export const mappingsActive = derived(
  [bindings, sendingEnabled],
  ([$b, $on]) => $on && $b.length > 0
);
