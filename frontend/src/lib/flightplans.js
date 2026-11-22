/**
 * Flight planning store: saved plans, the current draft, airport search and the
 * live navlog. The backend owns the calculations and exports; this module only
 * keeps the editor state and calls it.
 */
import { writable, derived, get } from 'svelte/store';
import { tNow } from './i18n.js';

export const plans = writable([]);
export const plansEnabled = writable(true);
export const selectedPlanID = writable(0);
export const draft = writable(blankPlan());
export const navlog = writable(null);
export const planError = writable('');
export const planLoading = writable(false);
export const planSaving = writable(false);
export const planDirty = writable(false);
export const navlogLoading = writable(false);

export const canSave = derived(draft, ($draft) =>
  $draft.name.trim().length > 0 && ($draft.route?.length ?? 0) >= 2);

export function blankPlan() {
  return {
    id: 0,
    name: '',
    aircraft: '',
    departure: '',
    arrival: '',
    cruiseAltFt: 35000,
    cruiseTasKt: 450,
    fuelBurnPph: 2500,
    reserveMin: 30,
    windFromDeg: 270,
    windKt: 20,
    flightRules: 'VFR',
    visibility: 'private',
    notes: '',
    route: [],
  };
}

export async function loadPlans() {
  planError.set('');
  try {
    const response = await fetch('/api/flightplans');
    if (!response.ok) throw new Error(`${response.status}`);
    const body = await response.json();
    if (body.enabled === false) {
      plansEnabled.set(false);
      plans.set([]);
      return;
    }
    plansEnabled.set(true);
    const list = body.plans ?? [];
    plans.set(list);
    const selected = get(selectedPlanID);
    if (selected && !list.some((plan) => plan.id === selected)) {
      selectedPlanID.set(0);
    }
    if (!get(selectedPlanID) && list.length > 0) {
      await selectPlan(list[0].id);
    }
  } catch (error) {
    planError.set(tNow('error.flightplan', { detail: error.message }));
  }
}

export async function selectPlan(id) {
  planLoading.set(true);
  planError.set('');
  try {
    const response = await fetch(`/api/flightplans/${id}`);
    if (!response.ok) throw new Error(`${response.status}`);
    const plan = await response.json();
    selectedPlanID.set(plan.id);
    draft.set(normalise(plan));
    planDirty.set(false);
    scheduleCalculate(true);
  } catch (error) {
    planError.set(tNow('error.flightplan', { detail: error.message }));
  } finally {
    planLoading.set(false);
  }
}

export function newPlan() {
  selectedPlanID.set(0);
  draft.set(blankPlan());
  navlog.set(null);
  planDirty.set(false);
  planError.set('');
}

export async function savePlan() {
  const plan = get(draft);
  planSaving.set(true);
  planError.set('');
  try {
    const response = await fetch(plan.id ? `/api/flightplans/${plan.id}` : '/api/flightplans', {
      method: plan.id ? 'PUT' : 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(plan),
    });
    const body = await response.json().catch(() => ({}));
    if (!response.ok) throw new Error(body.error || `${response.status}`);
    selectedPlanID.set(body.id);
    draft.set(normalise(body));
    planDirty.set(false);
    await loadPlans();
    scheduleCalculate(true);
  } catch (error) {
    planError.set(tNow('error.flightplan', { detail: error.message }));
  } finally {
    planSaving.set(false);
  }
}

export async function deletePlan(id) {
  planError.set('');
  try {
    const response = await fetch(`/api/flightplans/${id}`, { method: 'DELETE' });
    if (!response.ok) {
      const body = await response.json().catch(() => ({}));
      throw new Error(body.error || `${response.status}`);
    }
    newPlan();
    await loadPlans();
  } catch (error) {
    planError.set(tNow('error.flightplan', { detail: error.message }));
  }
}

export function updatePlan(patch) {
  draft.update((plan) => normalise({ ...plan, ...patch }));
  planDirty.set(true);
  scheduleCalculate();
}

/** Replaces the departure and keeps the rest of the route. */
export function setDeparture(airport) {
  routeUpdate((route) => {
    const point = waypointFromAirport(airport);
    if (route.length === 0) route.push(point);
    else route[0] = point;
  });
}

/** Appends or replaces the arrival. The UI disables it until a departure exists. */
export function setArrival(airport) {
  routeUpdate((route) => {
    if (route.length === 0) return;
    const point = waypointFromAirport(airport);
    if (route.length === 1) route.push(point);
    else route[route.length - 1] = point;
  });
}

export function addAirport(airport) {
  routeUpdate((route) => {
    if (route.length < 2) return;
    const point = waypointFromAirport(airport);
    route.splice(Math.max(1, route.length - 1), 0, point);
  });
}

export function addPoint(point) {
  routeUpdate((route) => {
    if (route.length < 2) return;
    route.splice(Math.max(1, route.length - 1), 0, { ...point, kind: 'user' });
  });
}

export function removeWaypoint(index) {
  routeUpdate((route) => {
    if (index > 0 && index < route.length - 1) route.splice(index, 1);
  });
}

export function moveWaypoint(index, delta) {
  routeUpdate((route) => {
    const target = index + delta;
    if (index <= 0 || index >= route.length - 1) return;
    if (target <= 0 || target >= route.length - 1) return;
    const [item] = route.splice(index, 1);
    route.splice(target, 0, item);
  });
}

function routeUpdate(mutator) {
  draft.update((plan) => {
    const route = [...(plan.route ?? [])];
    mutator(route);
    return normalise({
      ...plan,
      route,
      departure: route[0]?.ident ?? '',
      arrival: route.length > 1 ? route[route.length - 1].ident : '',
    });
  });
  planDirty.set(true);
  scheduleCalculate();
}

function normalise(plan) {
  return {
    ...blankPlan(),
    ...plan,
    route: Array.isArray(plan.route) ? plan.route.map((point) => ({ ...point })) : [],
    cruiseAltFt: number(plan.cruiseAltFt),
    cruiseTasKt: number(plan.cruiseTasKt),
    fuelBurnPph: number(plan.fuelBurnPph),
    reserveMin: number(plan.reserveMin),
    windFromDeg: number(plan.windFromDeg),
    windKt: number(plan.windKt),
  };
}

function number(value) {
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : 0;
}

function waypointFromAirport(airport) {
  return {
    ident: airport.ident ?? '',
    name: airport.name ?? '',
    kind: 'airport',
    icao: airport.icao ?? '',
    source: airport.source ?? 'ourairports',
    lat: number(airport.lat),
    lon: number(airport.lon),
    elevationFt: number(airport.elevationFt),
  };
}

export async function searchAirports(query, theatre) {
  const text = (query ?? '').trim();
  if (text.length < 1) return [];
  try {
    const params = new URLSearchParams({ q: text });
    if (theatre) params.set('theatre', theatre);
    const response = await fetch(`/api/airports?${params.toString()}`);
    if (!response.ok) throw new Error(`${response.status}`);
    const body = await response.json();
    return body.airports ?? [];
  } catch {
    return [];
  }
}
let calculateTimer;
let calculateSeq = 0;
export function scheduleCalculate(immediate = false) {
  clearTimeout(calculateTimer);
  calculateTimer = setTimeout(calculate, immediate ? 0 : 350);
}

async function calculate() {
  const plan = get(draft);
  if (!plan.route || plan.route.length < 2) {
    navlog.set(null);
    return;
  }
  const seq = ++calculateSeq;
  navlogLoading.set(true);
  try {
    const response = await fetch('/api/flightplans/calculate', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(plan),
    });
    const body = await response.json().catch(() => ({}));
    if (!response.ok) throw new Error(body.error || `${response.status}`);
    if (seq === calculateSeq) {
      navlog.set(body.navlog ?? null);
      planError.set('');
    }
  } catch (error) {
    if (seq === calculateSeq) {
      navlog.set(null);
      planError.set(tNow('error.flightplan', { detail: error.message }));
    }
  } finally {
    if (seq === calculateSeq) navlogLoading.set(false);
  }
}

export function briefingURL(id, lang) {
  return `/api/flightplans/${id}/briefing?lang=${encodeURIComponent(lang ?? 'en')}`;
}
