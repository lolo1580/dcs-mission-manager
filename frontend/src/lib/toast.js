import { writable } from 'svelte/store';

export const toasts = writable([]);
let nextID = 1;

/** Show a short confirmation or error in the corner of the window. */
export function showToast(message, tone = 'info', duration = 3600) {
  const id = nextID++;
  toasts.update((list) => [...list, { id, message, tone }]);
  if (typeof window !== 'undefined') {
    window.setTimeout(() => dismissToast(id), duration);
  }
  return id;
}

export function dismissToast(id) {
  toasts.update((list) => list.filter((toast) => toast.id !== id));
}
