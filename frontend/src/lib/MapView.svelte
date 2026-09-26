<script>
  import { onMount, onDestroy } from 'svelte';
  import L from 'leaflet';
  import 'leaflet/dist/leaflet.css';  import {
    visibleUnits,
    filters,
    connect,
    connected,
    lastUpdate,
    selectedId,
    selectedUnit,
    units,
    fetchTheatres,
    basemapId,
    basemaps as basemapsStore,
    theatres,
    theatre,
    setTheatre,
    showBounds,
  } from './units.js';
  import { unitIcon, coalitionColor } from './icons.js';
  import { heatPoints, trails as trailsStore, heatSource, maxHeatWeight } from './analytics.js';
  import {
    aerodromes,
    showOnMap as showAerodromes,
    selectAerodrome,
    towns,
    showTowns,
  } from './aerodromes.js';
  import { focusRequest } from './ui.js';

  let mapEl;
  let map;
  /** @type {Map<string, {marker: any, trail: any, trailPts: Array<[number,number]>}>} */
  const layers = new Map();
  let trailLayer;
  let historyLayer;
  let heatLayer;
  let aerodromeLayer;
  let townLayer;
  let boundsLayer;
  let userMoved = false;
  let followOwnship = true;

  /** @type {Array<{id:string,name:string,url:string,attribution:string,maxZoom:number,subdomains?:string[]}>} */
  let basemaps = [];
  let dcsTiles = null; // {theatre, bounds} when authentic DCS tiles exist
  let currentLayer = null;
  /** Theatre whose bounds have already been framed, so a manual pan is kept. */
  let framedTheatre = null;
  /** Airfield to focus once the map exists (set before mount completes). */
  let pendingAerodrome = null;

  // History overlays (heatmap and stored trails). Controlled by the parent via
  // the `history` prop, so the state survives switching tabs.
  export let history = false;
  let mapReady = false;

  const TRAIL_MAX = 120;

  onMount(async () => {
    map = L.map(mapEl, { zoomControl: true, preferCanvas: true }).setView([45, 40], 5);
    trailLayer = L.layerGroup().addTo(map);
    historyLayer = L.layerGroup();
    heatLayer = L.layerGroup();
    aerodromeLayer = L.layerGroup();
    townLayer = L.layerGroup();
    boundsLayer = L.layerGroup();

    try {
      const meta = await fetchTheatres();
      basemaps = meta.basemaps ?? [];
      if (meta.default && !localStorage.getItem('dcsmm.theatre')) {
        theatre.set(meta.default);
      }
      if (meta.basemap && basemaps.some((b) => b.id === meta.basemap) && !localStorage.getItem('dcsmm.basemap')) {
        basemapId.set(meta.basemap);
      }
    } catch {
      basemaps = [
        { id: 'osm', name: 'Routier', url: 'https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', attribution: '© OpenStreetMap', maxZoom: 19, subdomains: ['a', 'b', 'c'] },
      ];
      basemapsStore.set(basemaps);
    }

    applyBasemap($basemapId ?? 'osm');
    basemapsStore.set(basemaps);
    const unsubBase = basemapId.subscribe(applyBasemap);

    // Stop auto-following as soon as the user pans or zooms manually.
    map.on('dragstart', () => (userMoved = true));
    map.on('zoomstart', (e) => {
      if (e.originalEvent) userMoved = true;
    });

    const stop = connect();
    const unsub = visibleUnits.subscribe(syncMarkers);
    const unsubHeat = heatPoints.subscribe(() => renderHistory());
    const unsubTrails = trailsStore.subscribe(() => renderHistory());
    const unsubAeroList = aerodromes.subscribe(() => renderAerodromes());
    const unsubAeroToggle = showAerodromes.subscribe(() => renderAerodromes());
    const unsubTowns = towns.subscribe(() => renderTowns());
    const unsubTownsToggle = showTowns.subscribe(() => renderTowns());
    const unsubBounds = showBounds.subscribe(() => renderBounds());
    const unsubTheatre = theatre.subscribe(() => onTheatreChange());
    const unsubFocus = focusRequest.subscribe((a) => {
      if (a) {
        focusAerodrome(a);
        focusRequest.set(null);
      }
    });

    mapReady = true;
    applyHistory();
    onTheatreChange();
    if (pendingAerodrome) focusAerodrome(pendingAerodrome);

    return () => {
      unsub?.();
      unsubBase?.();
      unsubHeat?.();
      unsubTrails?.();
      unsubAeroList?.();
      unsubAeroToggle?.();
      unsubTowns?.();
      unsubTownsToggle?.();
      unsubBounds?.();
      unsubTheatre?.();
      unsubFocus?.();
      stop?.();
      map?.remove();
    };
  });

  /** Reacts to a theatre change: basemap availability, extent, airfields. */
  function onTheatreChange() {
    if (!map) return;
    const th = $theatres.find((t) => t.id === $theatre);

    // Authentic DCS tiles, when the theatre provides them.
    dcsTiles = th?.tiles ? { theatre: th.id, bounds: th.bounds } : null;
    if (th?.tiles && !basemaps.some((b) => b.id === 'dcs')) {
      basemaps = [
        { id: 'dcs', name: 'DCS (official)', url: `/api/tiles/${th.id}/{z}/{x}/{y}.png`, attribution: 'DCS World', maxZoom: 8 },
        ...basemaps,
      ];
      basemapsStore.set(basemaps);
    }

    // Frame the map on the theatre the first time it becomes active, so the
    // view is not lost when the user then pans or zooms by hand.
    if (th?.bounds && framedTheatre !== th.id) {
      frameBounds(th.bounds);
      framedTheatre = th.id;
    }

    renderBounds();
    loadTheatreAerodromes(th?.id);
  }

  /** Fits the map to a theatre's bounding box. */
  function frameBounds(b) {
    if (!map || !b) return;
    const south = Math.max(b.minLat, -90);
    const north = Math.min(b.maxLat, 90);
    const west = Math.max(b.minLng, -180);
    const east = Math.min(b.maxLng, 180);
    map.fitBounds(L.latLngBounds([south, west], [north, east]).pad(0.05));
  }

  /** Outlines the DCS map extent when the toggle is on. */
  function renderBounds() {
    if (!map) return;
    boundsLayer.clearLayers();

    let on = false;
    showBounds.subscribe((v) => (on = v))();
    const b = $theatre ? $theatres.find((t) => t.id === $theatre)?.bounds : null;
    if (!on || !b) {
      map.removeLayer(boundsLayer);
      return;
    }
    boundsLayer.addTo(map);

    const name = $theatres.find((t) => t.id === $theatre)?.name ?? $theatre;
    L.rectangle(
      [
        [b.minLat, b.minLng],
        [b.maxLat, b.maxLng],
      ],
      { color: '#f0b429', weight: 1.5, dashArray: '6 4', fill: false, interactive: false }
    )
      .bindTooltip(`${name} — DCS map extent`, { sticky: true })
      .addTo(boundsLayer);
  }

  /** Loads the airfields of the active theatre (or all of them if unknown). */
  function loadTheatreAerodromes(id) {
    const q = id ? `?theatre=${encodeURIComponent(id)}` : '';
    fetch(`/api/aerodromes${q}`)
      .then((r) => (r.ok ? r.json() : Promise.reject(new Error(`${r.status}`))))
      .then((body) => aerodromes.set(body.aerodromes ?? []))
      .catch(() => {
        /* the Airfields tab surfaces load errors */
      });
  }

  /** Draws airfield markers when the toggle is on. */
  function renderAerodromes() {
    if (!map) return;
    aerodromeLayer.clearLayers();

    let on = false;
    showAerodromes.subscribe((v) => (on = v))();
    if (!on) {
      map.removeLayer(aerodromeLayer);
      return;
    }
    aerodromeLayer.addTo(map);

    let list = [];
    aerodromes.subscribe((v) => (list = v))();

    for (const a of list) {
      const html = `
        <svg viewBox="0 0 24 24" width="18" height="18" fill="none"
             stroke="#f0b429" stroke-width="2" stroke-linecap="round">
          <circle cx="12" cy="12" r="9"/>
          <path d="M12 3 v18 M3 12 h18"/>
        </svg>`;
      const icon = L.divIcon({ html, className: 'dcsmm-marker', iconSize: [18, 18], iconAnchor: [9, 9] });
      L.marker([a.lat, a.lng], { icon })
        .bindTooltip(
          `<strong>${a.name}</strong> (${a.id})<br/>` +
            (a.tower ? `Tower ${a.tower.toFixed(3)} MHz<br/>` : '') +
            (a.tacan ? `TACAN ${a.tacan}<br/>` : '') +
            (a.ils?.length ? `ILS ${a.ils.map((i) => `${i.runway} ${i.mhz}`).join(', ')}` : '')
        )
        // Clicking an airfield opens its full data card, in the corner the unit
        // card uses. The two never show at once (see UnitDetails).
        .on('click', () => {
          selectedId.set(null);
          selectAerodrome(a);
        })
        .addTo(aerodromeLayer);
    }
  }

  /** Draws settlements when the toggle is on. Thousands of points, so a canvas
   *  circle marker is used rather than a DOM marker. */
  function renderTowns() {
    if (!map) return;
    townLayer.clearLayers();

    let on = false;
    showTowns.subscribe((v) => (on = v))();
    if (!on) {
      map.removeLayer(townLayer);
      return;
    }
    townLayer.addTo(map);
    // Settlements are context, not content: keep them behind the units.
    townLayer.bringToBack();

    let list = [];
    towns.subscribe((v) => (list = v))();
    for (const t of list) {
      L.circleMarker([t.lat, t.lng], {
        radius: 2,
        stroke: false,
        fillColor: '#e8e2d0',
        fillOpacity: 0.55,
        interactive: true,
      })
        .bindTooltip(t.name, { direction: 'top' })
        .addTo(townLayer);
    }
  }

  // Render the history overlays whenever the toggle or the data changes.
  $: if (mapReady) applyHistory();

  function applyHistory() {
    if (!map) return;
    if (history) {
      heatLayer.addTo(map);
      historyLayer.addTo(map);
    } else {
      map.removeLayer(heatLayer);
      map.removeLayer(historyLayer);
    }
    renderHistory();
  }

  /** Fills the heatmap and trail layers from the analytics stores. */
  function renderHistory() {
    if (!map) return;
    heatLayer.clearLayers();
    historyLayer.clearLayers();
    if (!history) return;

    let pts = [];
    heatPoints.subscribe((v) => (pts = v))();
    let max = 1;
    for (const p of pts) max = Math.max(max, p.weight ?? 0);
    for (const p of pts) {
      const ratio = (p.weight ?? 0) / max;
      L.circleMarker([p.lat, p.lng], {
        radius: 4 + ratio * 14,
        stroke: false,
        fillColor: heatColor(ratio),
        fillOpacity: 0.45,
      }).addTo(heatLayer);
    }

    let byUnit = {};
    trailsStore.subscribe((v) => (byUnit = v ?? {}))();
    for (const [unitId, trail] of Object.entries(byUnit)) {
      if (!Array.isArray(trail) || trail.length < 2) continue;
      L.polyline(
        trail.map((p) => [p.lat, p.lng]),
        { color: '#f0b429', weight: 1.5, opacity: 0.5 }
      )
        .bindTooltip(unitId)
        .addTo(historyLayer);
    }
  }

  /** Heat colour ramp: cool (low) to hot (high). */
  function heatColor(ratio) {
    const hue = 220 - 220 * Math.min(Math.max(ratio, 0), 1); // 220 blue -> 0 red
    return `hsl(${hue}, 90%, 55%)`;
  }

  function applyBasemap(id) {
    if (!map) return;
    const bm = basemaps.find((b) => b.id === id) ?? basemaps[0];
    if (!bm) return;
    if (currentLayer) map.removeLayer(currentLayer);
    currentLayer = L.tileLayer(bm.url, {
      attribution: bm.attribution,
      maxZoom: bm.maxZoom ?? 19,
      subdomains: bm.subdomains ?? 'abc',
      className: bm.className ?? '',
      // DCS tile sets are only available at low zoom levels.
      minZoom: bm.id === 'dcs' ? 0 : 0,
      errorTileUrl: 'data:image/gif;base64,R0lGODlhAQABAAAAACH5BAEKAAEALAAAAAABAAEAAAICTAEAOw==',
    }).addTo(map);
    currentLayer.bringToBack();
  }

  function syncMarkers(list) {
    const seen = new Set();

    for (const u of list) {
      seen.add(u.id);
      let entry = layers.get(u.id);
      if (!entry) {
        const marker = L.marker([u.lat, u.lng], {
          icon: unitIcon(L, u),
          riseOnHover: true,
        }).addTo(map);
        marker.on('click', () => selectedId.set(u.id));
        marker.bindTooltip('', { direction: 'top', offset: [0, -8] });
        entry = { marker, iconKey: iconKey(u), trailPts: [] };
        layers.set(u.id, entry);
      }

      entry.marker.setLatLng([u.lat, u.lng]);

      // Recreating the icon on every tick would be wasteful with hundreds of
      // units; only do it when the visual identity actually changes.
      const key = iconKey(u);
      if (key !== entry.iconKey) {
        entry.marker.setIcon(unitIcon(L, u));
        entry.iconKey = key;
      }
      entry.marker.setTooltipContent(tooltip(u));

      // Flight trails (planes and helicopters only). Stored per entry rather
      // than on the Leaflet options object.
      if ($filters.showTrails && (u.category === 'plane' || u.category === 'heli')) {
        const pts = entry.trailPts;
        const last = pts[pts.length - 1];
        if (!last || last[0] !== u.lat || last[1] !== u.lng) {
          pts.push([u.lat, u.lng]);
          if (pts.length > TRAIL_MAX) pts.shift();
        }
        entry.trailColor = coalitionColor(u.coalition);
      } else {
        entry.trailPts = [];
      }
      entry.type = u.type;
    }

    // Remove markers for units that vanished.
    for (const [id, entry] of layers) {
      if (!seen.has(id)) {
        entry.marker.remove();
        layers.delete(id);
      }
    }

    renderTrails();
    maybeFollow();
  }

  function iconKey(u) {
    return `${u.category}|${u.coalition}|${u.ownship ? 1 : 0}`;
  }

  function renderTrails() {
    trailLayer.clearLayers();
    for (const entry of layers.values()) {
      const pts = entry.trailPts;
      if (pts && pts.length > 1) {
        L.polyline(pts, {
          color: entry.trailColor,
          weight: 1.5,
          opacity: 0.45,
        }).addTo(trailLayer);
      }
    }
  }

  function maybeFollow() {
    if (!userMoved && followOwnship) {
      const own = $selectedUnit || $units.find((u) => u.ownship);
      if (own) map.panTo([own.lat, own.lng], { animate: true, duration: 0.5 });
    }
  }

  function tooltip(u) {
    const label = u.label || u.type;
    return `<strong>${label}</strong><br/>${u.type}<br/>alt ${Math.round(u.alt)} m · cap ${Math.round(u.heading)}°`;
  }

  export function recenter() {
    userMoved = false;
    const own = $units.find((u) => u.ownship);
    if (own) {
      map.setView([own.lat, own.lng], 8);
    } else if ($visibleUnits.length) {
      map.fitBounds(L.latLngBounds($visibleUnits.map((u) => [u.lat, u.lng])).pad(0.3));
    } else {
      const b = $theatre ? $theatres.find((t) => t.id === $theatre)?.bounds : null;
      if (b) {
        framedTheatre = $theatre;
        frameBounds(b);
      } else if (dcsTiles) {
        frameBounds(dcsTiles.bounds);
      }
    }
  }

  /** Focuses one airfield: used by the Airfields tab to reveal an airfield. */
  export function focusAerodrome(a) {
    if (!a) return;
    // The caller may switch to the map tab in the same tick, before this
    // component has finished mounting: remember the target and apply it below.
    if (!map) {
      pendingAerodrome = a;
      return;
    }
    userMoved = true;
    map.setView([a.lat, a.lng], 10);
    pendingAerodrome = null;
  }
</script>

<div class="map" bind:this={mapEl}></div>

<style>
  .map {
    width: 100%;
    height: 100%;
    background: #0b0f14;
  }

  :global(.dcsmm-marker) {
    background: transparent;
    border: none;
  }

  /* Key-free dark mode: invert and hue-rotate the standard raster tiles. */
  :global(.dcsmm-dark-tiles) {
    filter: invert(1) hue-rotate(180deg) brightness(0.9) contrast(0.95) saturate(0.7);
  }

  :global(.leaflet-container) {
    background: #0b0f14;
    font: inherit;
  }
</style>
