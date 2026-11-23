/**
 * Minimal history router.
 *
 * The application is an SPA embedded in the Go binary; the Go handler already
 * falls back to index.html for unknown paths, so pushState routes work without
 * a hash. Routes are declared once here and reused by the shell and the
 * document title.
 */
import { writable } from 'svelte/store';

export const NAV_GROUPS = [
  {
    label: 'nav.fly',
    items: [
      { id: 'overview', path: '/overview', label: 'tab.overview', subtitle: 'sub.overview', icon: 'M4 13h6V4H4zM14 20h6v-9h-6zM4 20h6v-3H4zM14 7h6V4h-6z' },
      { id: 'flights', path: '/flights', label: 'tab.flights', subtitle: 'sub.flights', icon: 'M3 12h18M12 3v18M5.6 5.6l12.8 12.8M18.4 5.6L5.6 18.4' },
      { id: 'career', path: '/career', label: 'tab.career', subtitle: 'sub.career', icon: 'M12 3l2.5 5 5.5.8-4 3.9 1 5.5-5-2.6-5 2.6 1-5.5-4-3.9 5.5-.8z' },
      { id: 'stats', path: '/stats', label: 'tab.stats', subtitle: 'sub.stats', icon: 'M4 20V10M10 20V4M16 20v-7M22 20H2' },
    ],
  },
  {
    label: 'nav.plan',
    items: [
      { id: 'checklists', path: '/checklists', label: 'tab.checklists', subtitle: 'sub.checklists', icon: 'M5 4h14v16H5zM8 9h8M8 13h8M8 17h5' },
    ],
  },
  {
    label: 'nav.reference',
    items: [
      { id: 'airfields', path: '/airfields', label: 'tab.aerodromes', subtitle: 'sub.aerodromes', icon: 'M12 3v6M12 15v6M3 12h6M15 12h6' },
    ],
  },
  {
    label: 'nav.library',
    items: [
      { id: 'mods', path: '/mods', label: 'tab.mods', subtitle: 'sub.mods', icon: 'M4 7l8-4 8 4-8 4zM4 12l8 4 8-4M4 17l8 4 8-4' },
    ],
  },
  {
    label: 'nav.config',
    items: [
      { id: 'settings', path: '/settings', label: 'tab.settings', subtitle: 'sub.settings', icon: 'M12 2v3M12 19v3M2 12h3M19 12h3M5 5l2 2M17 17l2 2M19 5l-2 2M7 17l-2 2' },
    ],
  },
];

// The server profile shares the flight, planning and reference pages; it adds a
// squadron dashboard and account administration instead of the DCS-only pages.
export const SERVER_NAV_GROUPS = [
  {
    label: 'nav.fly',
    items: [
      { id: 'squadron', path: '/squadron', label: 'tab.squadron', subtitle: 'sub.squadron', icon: 'M3 20V9l9-6 9 6v11M8 20v-7h8v7' },
      { id: 'flights', path: '/flights', label: 'tab.flights', subtitle: 'sub.flights', icon: 'M3 12h18M12 3v18M5.6 5.6l12.8 12.8M18.4 5.6L5.6 18.4' },
      { id: 'stats', path: '/stats', label: 'tab.stats', subtitle: 'sub.stats', icon: 'M4 20V10M10 20V4M16 20v-7M22 20H2' },
    ],
  },
  {
    label: 'nav.plan',
    items: [
      { id: 'checklists', path: '/checklists', label: 'tab.checklists', subtitle: 'sub.checklists', icon: 'M5 4h14v16H5zM8 9h8M8 13h8M8 17h5' },
    ],
  },
  {
    label: 'nav.reference',
    items: [
      { id: 'airfields', path: '/airfields', label: 'tab.aerodromes', subtitle: 'sub.aerodromes', icon: 'M12 3v6M12 15v6M3 12h6M15 12h6' },
    ],
  },
  {
    label: 'nav.school',
    items: [
      { id: 'school', path: '/school', label: 'tab.school', subtitle: 'sub.school', icon: 'M4 19.5A2.5 2.5 0 0 1 6.5 17H20M4 19.5V6a2 2 0 0 1 2-2h14v13H6.5A2.5 2.5 0 0 0 4 19.5zM8 7h8M8 10h6' },
      { id: 'culture', path: '/culture', label: 'tab.culture', subtitle: 'sub.culture', icon: 'M12 3l9 5-9 5-9-5 9-5zM3 12l9 5 9-5M3 16l9 5 9-5' },
    ],
  },
  {
    label: 'nav.account',
    items: [
      { id: 'profile', path: '/profile', label: 'tab.profile', subtitle: 'sub.profile', icon: 'M12 12a4 4 0 1 0 0-8 4 4 0 0 0 0 8zM4 21a8 8 0 0 1 16 0' },
      { id: 'admin', path: '/admin', label: 'tab.admin', subtitle: 'sub.admin', icon: 'M12 3l8 3v6c0 5-3.4 8.2-8 9-4.6-.8-8-4-8-9V6z' },
    ],
  },
];

const ROUTES = new Map();
for (const group of [...NAV_GROUPS, ...SERVER_NAV_GROUPS]) {
  for (const item of group.items) ROUTES.set(item.id, item);
}

export function routeByID(id) {
  return ROUTES.get(id) ?? ROUTES.get('overview');
}

/** Resolve a pathname to a route id. Unknown paths fall back to Overview. */
export function idFromPath(pathname) {
  const clean = (pathname || '/').replace(/\/+$/, '') || '/';
  if (clean === '/' || clean === '/overview') return 'overview';
  for (const item of ROUTES.values()) {
    if (item.path === clean) return item.id;
  }
  return 'overview';
}

function currentID() {
  if (typeof window === 'undefined') return 'overview';
  return idFromPath(window.location.pathname);
}

export const routeID = writable(currentID());

/** Push a route without reloading the page. */
export function navigate(id) {
  const route = routeByID(id);
  if (typeof window !== 'undefined' && window.location.pathname !== route.path) {
    window.history.pushState({ route: route.id }, '', route.path);
  }
  routeID.set(route.id);
}

/** Install the popstate listener. Call once from the application shell. */
export function initRouter() {
  if (typeof window === 'undefined') return () => {};
  const onPop = () => routeID.set(currentID());
  window.addEventListener('popstate', onPop);
  // Normalise the first URL so the address bar matches the displayed page.
  const id = currentID();
  const route = routeByID(id);
  if (window.location.pathname !== route.path) {
    window.history.replaceState({ route: id }, '', route.path);
  }
  return () => window.removeEventListener('popstate', onPop);
}
