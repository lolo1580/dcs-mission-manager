/**
 * DCS-side install store: the mods found under Saved Games\DCS\Mods, and the
 * script status (our managed files, the tools merged into Export.lua, the
 * pre-rename leftovers, and other tools' hooks).
 */
import { writable, derived } from 'svelte/store';
import { tNow } from './i18n.js';

/** @type {import('svelte/store').Writable<Array>} */
export const mods = writable([]);
/** @type {import('svelte/store').Writable<object|null>} */
export const scriptStatus = writable(null);
export const installError = writable('');
export const installLoading = writable(false);

export async function loadInstall() {
  installLoading.set(true);
  installError.set('');
  try {
    const getJSON = async (url) => {
      const r = await fetch(url);
      if (!r.ok) throw new Error(`${url}: ${r.status}`);
      return r.json();
    };
    const [modsRes, scriptsRes] = await Promise.all([
      getJSON('/api/mods'),
      getJSON('/api/scripts'),
    ]);
    mods.set(modsRes.mods ?? []);
    scriptStatus.set(scriptsRes ?? null);
  } catch (e) {
    installError.set(tNow('error.install', { detail: e.message }));
  } finally {
    installLoading.set(false);
  }
}

/** Total size of every mod, in bytes. */
export const modsTotalBytes = derived(mods, ($m) =>
  $m.reduce((sum, x) => sum + (x.sizeBytes ?? 0), 0)
);

/** True when a managed file is not up to date. Drives a summary banner. */
export const installNeedsAttention = derived(scriptStatus, ($s) => {
  if (!$s) return false;
  return ($s.managed ?? []).some((m) => m.state === 'missing' || m.state === 'outdated');
});

/** Formats a file size as "1.2 MB" / "840 KB" / "8 GB". */
export function fmtSize(bytes) {
  if (typeof bytes !== 'number') return '—';
  if (bytes >= 1024 ** 3) return `${(bytes / 1024 ** 3).toFixed(1)} GB`;
  if (bytes >= 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  return `${Math.round(bytes / 1024)} KB`;
}
