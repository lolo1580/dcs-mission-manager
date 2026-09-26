/**
 * Shared UI navigation state.
 *
 * Panels need to be able to send the user somewhere else (for example "show this
 * airfield on the map"), without every component holding a reference to the map.
 * A store keeps that free of prop drilling.
 */
import { writable } from 'svelte/store';
import { selectAerodrome } from './aerodromes.js';

/** Active tab id ('map', 'session', 'debriefs', 'stats', 'analytics', …). */
export const activeTab = writable('map');

/**
 * Airfield the map should focus next. MapView subscribes, applies it and clears
 * it back to null, so a later mount can consume a request made before the map
 * existed.
 */
export const focusRequest = writable(null);

/**
 * Ask the map to reveal an airfield, switching to the map tab. The airfield is
 * also selected, so its data card opens on arrival.
 *
 * It goes through selectAerodrome rather than writing the store directly, so the
 * side effects (loading the airfield's charts) always run.
 */
export function revealAerodrome(a) {
  if (!a) return;
  selectAerodrome(a);
  activeTab.set('map');
  focusRequest.set(a);
}
