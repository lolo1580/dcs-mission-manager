import { writable } from 'svelte/store';

export const updateInfo = writable(null);
export const updateLoading = writable(false);

export async function checkForUpdates(refresh = false) {
  updateLoading.set(true);
  try {
    const response = await fetch(`/api/update${refresh ? '?refresh=1' : ''}`);
    if (!response.ok) throw new Error(`HTTP ${response.status}`);
    updateInfo.set(await response.json());
  } catch (_) {
    updateInfo.set({ status: 'error' });
  } finally {
    updateLoading.set(false);
  }
}
