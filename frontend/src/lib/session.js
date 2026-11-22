/**
 * Session store for the multi-user server profile.
 *
 * The desktop has no accounts and never asks for a login; in server mode the
 * application waits for `/api/health` (or the session endpoint) before showing
 * the shell. This store keeps the current user and exposes the two operations
 * the UI needs: login and logout.
 */
import { writable, derived, get } from 'svelte/store';

export const appMode = writable('desktop');
export const sessionUser = writable(null);
export const sessionLoading = writable(true);
export const sessionError = writable('');

export const isServer = derived(appMode, ($mode) => $mode === 'server');
export const isAdmin = derived(sessionUser, ($user) => $user?.role === 'admin');

export async function loadSession() {
  sessionLoading.set(true);
  sessionError.set('');
  try {
    const healthResponse = await fetch('/api/health', { headers: { Accept: 'application/json' } });
    if (!healthResponse.ok) throw new Error(`${healthResponse.status}`);
    const health = await healthResponse.json();
    appMode.set(health.mode ?? 'desktop');
    if (health.mode !== 'server') {
      sessionUser.set(null);
      return;
    }
    const response = await fetch('/api/auth/session', { headers: { Accept: 'application/json' } });
    if (!response.ok) throw new Error(`${response.status}`);
    const body = await response.json();
    sessionUser.set(body.user ?? null);
  } catch (error) {
    sessionError.set(error.message);
  } finally {
    sessionLoading.set(false);
  }
}

export async function login(username, password) {
  const response = await fetch('/api/auth/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
  });
  if (!response.ok) {
    let detail = `${response.status}`;
    try {
      const body = await response.json();
      if (body?.error) detail = body.error;
    } catch {
      /* keep the status */
    }
    throw new Error(detail);
  }
  const body = await response.json();
  sessionUser.set(body.user);
  return body.user;
}

export async function logout() {
  try {
    await fetch('/api/auth/logout', { method: 'POST' });
  } finally {
    sessionUser.set(null);
  }
}

export async function refreshSession() {
  if (get(appMode) !== 'server') return;
  const response = await fetch('/api/auth/session');
  if (!response.ok) return;
  const body = await response.json();
  sessionUser.set(body.user ?? null);
}
