/**
 * DCS configuration store: the game's own settings, read from
 * Saved Games\DCS\Config (options.lua groups, pluginsEnabled.lua, lang.cfg).
 */
import { writable, derived } from 'svelte/store';
import { tNow } from './i18n.js';

/** @type {import('svelte/store').Writable<object|null>} */
export const dcsConfig = writable(null);
export const configError = writable('');
export const configLoading = writable(false);

/** Which section is displayed; empty means the first one. */
export const configSection = writable('');

export async function loadDCSConfig() {
  configLoading.set(true);
  configError.set('');
  try {
    const res = await fetch('/api/config');
    if (!res.ok) throw new Error(`${res.status}`);
    const body = await res.json();
    dcsConfig.set(body ?? null);
  } catch (e) {
    configError.set(tNow('error.config', { detail: e.message }));
  } finally {
    configLoading.set(false);
  }
}

/** Section names, in DCS's own order. */
export const configSections = derived(dcsConfig, ($c) => ($c?.sections ?? []).map((s) => s.name));

/** The section currently shown: the explicit choice, else the first. */
export const currentSection = derived(
  [dcsConfig, configSection],
  ([$c, $want]) => {
    const list = $c?.sections ?? [];
    if (!list.length) return null;
    return list.find((s) => s.name === $want) ?? list[0];
  }
);

/** Only the plugins toggled OFF, which is what usually explains a missing map. */
export const disabledPlugins = derived(dcsConfig, ($c) =>
  ($c?.plugins ?? []).filter((p) => !p.enabled)
);
