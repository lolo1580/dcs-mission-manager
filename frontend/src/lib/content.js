/**
 * Mods & skins page store: the local mod/livery inventory, quarantine and the
 * archive import workflow.
 */
import { writable, derived } from 'svelte/store';
import { tNow } from './i18n.js';

export const contentStatus = writable(null);
export const contentModsScan = writable(null);
export const contentMods = writable([]);
export const contentLiveryScan = writable(null);
export const contentLiveries = writable([]);
export const contentQuarantine = writable([]);
export const contentLoading = writable(false);
export const contentBusy = writable(false);
export const contentError = writable('');

export const dcsRunning = derived(contentStatus, ($status) => Boolean($status?.dcsRunning));

async function getJSON(url) {
  const response = await fetch(url);
  const body = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(body.error || `${url}: ${response.status}`);
  return body;
}

async function postJSON(url, payload) {
  const response = await fetch(url, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  const body = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(body.error || `${url}: ${response.status}`);
  return body;
}

export async function loadContent() {
  contentLoading.set(true);
  contentError.set('');
  try {
    const [status, mods, liveries, quarantine] = await Promise.all([
      getJSON('/api/content/status'),
      getJSON('/api/content/mods'),
      getJSON('/api/content/liveries'),
      getJSON('/api/content/quarantine'),
    ]);
    contentStatus.set(status);
    contentModsScan.set(mods);
    contentMods.set(mods.mods ?? []);
    contentLiveryScan.set(liveries);
    contentLiveries.set(liveries.liveries ?? []);
    contentQuarantine.set(quarantine.items ?? []);
  } catch (error) {
    contentError.set(tNow('content.error.load', { detail: error.message }));
  } finally {
    contentLoading.set(false);
  }
}

export async function refreshAll() {
  await loadContent();
}

export async function modAction(action, payload) {
  contentBusy.set(true);
  contentError.set('');
  try {
    const result = await postJSON('/api/content/mods/action', { action, ...payload });
    await loadContent();
    return result;
  } catch (error) {
    contentError.set(error.message);
    throw error;
  } finally {
    contentBusy.set(false);
  }
}

export async function liveryAction(action, payload) {
  contentBusy.set(true);
  contentError.set('');
  try {
    const result = await postJSON('/api/content/liveries/action', { action, ...payload });
    await loadContent();
    return result;
  } catch (error) {
    contentError.set(error.message);
    throw error;
  } finally {
    contentBusy.set(false);
  }
}

export async function inspectArchive(file) {
  contentBusy.set(true);
  contentError.set('');
  try {
    const body = new FormData();
    body.append('file', file, file.name);
    const response = await fetch('/api/content/import/inspect', { method: 'POST', body });
    const data = await response.json().catch(() => ({}));
    if (!response.ok) throw new Error(data.error || `${response.status}`);
    return data;
  } catch (error) {
    contentError.set(error.message);
    throw error;
  } finally {
    contentBusy.set(false);
  }
}

export async function commitImport(id, selected, replace) {
  contentBusy.set(true);
  contentError.set('');
  try {
    const result = await postJSON('/api/content/import/commit', { id, selected, replace });
    await loadContent();
    return result;
  } catch (error) {
    contentError.set(error.message);
    throw error;
  } finally {
    contentBusy.set(false);
  }
}

export async function cancelImport(id) {
  try {
    await fetch(`/api/content/import/${encodeURIComponent(id)}`, { method: 'DELETE' });
  } catch {
    // The staging folder expires on its own; cancellation is best-effort.
  }
}

export function thumbnailURL(id) {
  return `/api/content/liveries/thumbnail?id=${encodeURIComponent(id)}`;
}
