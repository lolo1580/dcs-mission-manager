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
  import { vectorLayers, shownVectors, vectorURL, loadVectors, overlaysFixed } from './vectors.js';

  /** True when the aeronautical style is active: chart-like labels are used. */
  let aero = false;
  /** Current zoom, so labels appear progressively as the map is zoomed in. */
  let zoom = 5;

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
  /** Rendered DCS vector layers, keyed by layer name, reused across toggles. */
  const vectorShapes = new Map();
  let userMoved = false;
  let followOwnship = true;

  /** @type {Array<{id:string,name:string,url:string,attribution:string,maxZoom:number,subdomains?:string[]}>} */
  let basemaps = [];
  let dcsTiles = null; // {theatre, bounds} when authentic DCS tiles exist
  let currentLayer = null;
  /** Credit for imported map tiles, from /api/theatres. Set in onMount. */
  let tilesAttribution = '';
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
      tilesAttribution = meta.tilesAttribution ?? '';
      // A basemap that no longer exists (an earlier release offered Satellite,
      // Relief and Road) must not leave the map on a layer it cannot draw.
      const stored = localStorage.getItem('dcsmm.basemap');
      if (stored && !basemaps.some((b) => b.id === stored)) {
        localStorage.removeItem('dcsmm.basemap');
        basemapId.set(meta.basemap && basemaps.some((b) => b.id === meta.basemap) ? meta.basemap : basemaps[0]?.id ?? 'aero');
      }
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

    applyBasemap($basemapId ?? 'aero');
    basemapsStore.set(basemaps);
    const unsubBase = basemapId.subscribe(applyBasemap);
    // Stop auto-following as soon as the user pans or zooms manually.
    map.on('dragstart', () => (userMoved = true));
    map.on('zoomstart', (e) => {
      if (e.originalEvent) userMoved = true;
    });

    // Airfield labels are zoom-dependent: showing them all at low zoom turns the
    // map into a pile of overlapping boxes.
    zoom = map.getZoom();
    map.on('zoomend', () => {
      zoom = map.getZoom();
      renderAerodromes();
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
    const unsubVectors = shownVectors.subscribe(() => renderVectors());
    const unsubVectorList = vectorLayers.subscribe(() => renderVectors());
    const unsubFixed = overlaysFixed.subscribe(() => {
      renderVectors();
      renderAerodromes();
    });
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
      unsubVectors?.();
      unsubVectorList?.();
      unsubFixed?.();
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

    // Authentic DCS tiles, when the theatre provides them. The zoom ceiling
    // comes from what is actually on disk, so a detailed pack is not capped.
    dcsTiles = th?.tiles ? { theatre: th.id, bounds: th.bounds } : null;
    if (th?.tiles) {
      const minZoom = th.tileMinZoom > 0 ? th.tileMinZoom : 0;
      const maxZoom = th.tileMaxZoom > 0 ? th.tileMaxZoom : 8;
      const credit = tilesAttribution || 'DCS map tiles';
      const existing = basemaps.find((b) => b.id === 'dcs');
      if (existing) {
        existing.url = `/api/tiles/${th.id}/{z}/{x}/{y}.png`;
        existing.maxZoom = maxZoom;
        existing.minZoom = minZoom;
        existing.attribution = credit;
        existing.bounds = th.bounds;
      } else {
        basemaps = [
          { id: 'dcs', name: 'DCS (official)', url: `/api/tiles/${th.id}/{z}/{x}/{y}.png`, attribution: credit, maxZoom, minZoom, bounds: th.bounds },
          ...basemaps,
        ];
      }
      basemapsStore.set(basemaps);

      // The DCS basemap only becomes available once the theatre list is known,
      // which is after the first applyBasemap ran. If the user's stored choice
      // is 'dcs', that first run fell back to the default basemap (the entry did
      // not exist yet) and nothing re-applied it: the selector said "DCS" while
      // the map still showed another layer. Re-apply now that it exists.
      if ($basemapId === 'dcs') {
        applyBasemap('dcs');
      }

      // A pack that starts at zoom 8 shows nothing at zoom 6: if the map is
      // currently wider than the pack, the basemap would look empty. Bring the
      // view into the range the tiles actually cover.
      if (map.getZoom() < minZoom) {
        map.setZoom(minZoom);
      }
    }

    // Frame the map on the theatre the first time it becomes active, so the
    // view is not lost when the user then pans or zooms by hand.
    if (th?.bounds && framedTheatre !== th.id) {
      frameBounds(th.bounds);
      framedTheatre = th.id;
    }
    // Terrain vectors are per theatre: reload the list when it changes.
    loadVectors();

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
      .bindTooltip(escapeHTML(name) + ' — DCS map extent', { sticky: true })
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

    // When the overlays are pinned, the toggle is bypassed: the layer is simply
    // always drawn, and the UI hides the button that would turn it off.
    let fixed = false;
    overlaysFixed.subscribe((v) => (fixed = v))();
    let on = false;
    showAerodromes.subscribe((v) => (on = v))();
    if (!fixed && !on) {
      map.removeLayer(aerodromeLayer);
      return;
    }
    aerodromeLayer.addTo(map);

    let list = [];
    aerodromes.subscribe((v) => (list = v))();

    for (const a of list) {
      // Colour matters here: the DCS terrain layers draw roads in orange and
      // urban areas in yellow, and the basemaps range from pale tan to dark
      // blue. An amber symbol vanished against most of them. Cyan is used
      // nowhere else on the map, and the dark halo under it keeps the marker
      // readable on a light background too.
      const html = `
        <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke-linecap="round">
          <g stroke="#0b1017" stroke-width="4.6" opacity=".85">
            <circle cx="12" cy="12" r="9"/>
            <path d="M12 3 v18 M3 12 h18"/>
          </g>
          <g stroke="#22d3ee" stroke-width="2.2">
            <circle cx="12" cy="12" r="9"/>
            <path d="M12 3 v18 M3 12 h18"/>
          </g>
        </svg>`;
      const icon = L.divIcon({ html, className: 'dcsmm-marker', iconSize: [20, 20], iconAnchor: [10, 10] });

      // On the aeronautical style, airfields carry a permanent label like a
      // chart; elsewhere a tooltip on hover is enough.
      const label =
        `<strong>${escapeHTML(a.name)}</strong>` +
        (a.icaoCode && a.icaoCode.length === 4 ? ` <span class="icao">${escapeHTML(a.icaoCode)}</span>` : '') +
        (a.tower ? `<br/>TWR ${a.tower.toFixed(3)}` : '') +
        (a.tacan ? ` · ${escapeHTML(a.tacan)}` : '') +
        (a.ils?.length ? `<br/>ILS ${a.ils.map((i) => `${i.runway ? i.runway + ' ' : ''}${i.mhz}`).join(', ')}` : '');

      // Chart-style labels are permanent, but only once the zoom is high enough
      // to fit them: below minLabelZoom the map would be unreadable.
      const labelled = aero && zoom >= 8;
      // Only the fields with navigation data are worth labelling when zoomed
      // out; at higher zoom every field gets its callout.
      const hasNavaid = Boolean(a.tower || a.tacan || a.ils?.length || a.vor);
      const showLabel = labelled && (zoom >= 9 || hasNavaid);

      L.marker([a.lat, a.lng], { icon })
        .bindTooltip(
          label,
          showLabel
            ? { permanent: true, direction: 'right', offset: [10, 0], className: 'dcsmm-chart-label' }
            : { direction: 'top' }
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

  /** Minimal HTML escaping: airfield names come from a file on disk. */
  function escapeHTML(s) {
    return String(s ?? '').replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c]);
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
        .bindTooltip(escapeHTML(t.name), { direction: 'top' })
        .addTo(townLayer);
    }
  }

  // Render the history overlays whenever the toggle or the data changes.
  $: if (mapReady) applyHistory();

  /**
   * Draws the DCS terrain vectors (roads, rivers, urban areas…) over the map.
   * Each layer is a GeoJSON fetch; the shapes are cached once fetched, so
   * toggling a layer on and off does not re-download it.
   */
  function renderVectors() {
    if (!map) return;
    let shown = new Set();
    shownVectors.subscribe((v) => (shown = v))();
    let layers = [];
    vectorLayers.subscribe((v) => (layers = v))();
    // Pinned overlays ignore the per-layer selection: every layer is drawn.
    let fixed = false;
    overlaysFixed.subscribe((v) => (fixed = v))();

    for (const layer of layers) {
      const want = fixed || shown.has(layer.name);
      let entry = vectorShapes.get(layer.name);
      if (!want) {
        if (entry) map.removeLayer(entry);
        continue;
      }
      if (entry) {
        entry.addTo(map);
        entry.bringToFront();
        continue;
      }
      // First time this layer is shown: fetch it, then attach.
      fetch(vectorURL(layer.file))
        .then((r) => (r.ok ? r.json() : Promise.reject(new Error(String(r.status)))))
        .then((geojson) => {
          const group = L.geoJSON(geojson, {
            style: () => vectorStyle(layer.name),
            // Markers are drawn as small dots; the polygons and lines carry the
            // meaning, and thousands of labels would bury the map.
            pointToLayer: (_, latlng) =>
              L.circleMarker(latlng, {
                radius: 2,
                stroke: false,
                fillColor: vectorStyle(layer.name).color,
                fillOpacity: 0.7,
                interactive: false,
              }),
            interactive: false,
          });
          vectorShapes.set(layer.name, group);
          // The layer may have been switched off (or the mode changed) while it
          // was downloading, so re-read both sources of truth here.
          let stillWanted = false;
          shownVectors.subscribe((v) => (stillWanted = v.has(layer.name)))();
          let stillFixed = false;
          overlaysFixed.subscribe((v) => (stillFixed = v))();
          if (stillWanted || stillFixed) {
            group.addTo(map);
            group.bringToFront();
          }
        })
        .catch(() => {
          /* a missing or malformed layer is skipped, not fatal */
        });
    }
  }

  /** Style for one DCS vector layer. */
  function vectorStyle(name) {
    const n = name.toLowerCase();
    // The palette deliberately avoids amber/yellow: that family is the
    // airfield symbol's job, and two meanings for one colour is what made the
    // markers unreadable in the first place.
    if (n.includes('waterbed')) return { color: '#7fb2d9', weight: 0, fillColor: '#4f83b3', fillOpacity: 0.35 };
    if (n.includes('rivers')) return { color: '#6aa6d6', weight: 1, opacity: 0.85, fill: false };
    if (n.includes('urban')) return { color: '#cbd5e1', weight: 0, fillColor: '#cbd5e1', fillOpacity: 0.18 };
    if (n.includes('borders')) return { color: '#e05252', weight: 2, dashArray: '8 5', opacity: 0.9, fill: false };
    if (n.includes('railroad')) return { color: '#b07d3a', weight: 1.2, dashArray: '3 3', opacity: 0.85, fill: false };
    if (n.includes('roads')) return { color: '#ff8c42', weight: 1.4, opacity: 0.9, fill: false };
    if (n.includes('airbase')) return { color: '#22d3ee', weight: 1.5, fillColor: '#22d3ee', fillOpacity: 0.15 };
    return { color: '#cccccc', weight: 1, opacity: 0.7, fill: false };
  }

  $: if (mapReady) renderVectors();

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
        .bindTooltip(escapeHTML(unitId))
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

    // A tile layer cannot exceed the map's own limits, and a pack only has the
    // levels that were imported. Apply both, so selecting a detailed pack is not
    // capped by the previous basemap and does not fall below the pack's floor.
    if (bm.maxZoom && map.getMaxZoom() !== bm.maxZoom) {
      map.setMaxZoom(bm.maxZoom);
    }
    map.setMinZoom(bm.minZoom ?? 0);

    // A pack covers one rectangle: keep the view inside it, so panning never
    // shows empty space beyond the tiles. Other basemaps release the limit.
    if (bm.bounds) {
      const box = L.latLngBounds(
        [bm.bounds.minLat, bm.bounds.minLng],
        [bm.bounds.maxLat, bm.bounds.maxLng],
      );
      map.setMaxBounds(box.pad(0.02));
      if (!box.contains(map.getBounds()) || map.getZoom() < (bm.minZoom ?? 0)) {
        map.fitBounds(box, { maxZoom: bm.maxZoom });
      }
    } else {
      map.setMaxBounds(null);
    }

    // The aeronautical style is meant to show airfields: switch them on, and use
    // permanent chart-style labels rather than hover tooltips.
    aero = bm.id === 'aero';
    if (aero) {
      showAerodromes.set(true);
    }
    renderAerodromes();

    currentLayer = L.tileLayer(bm.url, {
      attribution: bm.attribution,
      maxZoom: bm.maxZoom ?? 19,
      subdomains: bm.subdomains ?? 'abc',
      className: bm.className ?? '',
      // Tiles are addressed top-left (XYZ), which is what the importer writes;
      // Leaflet's default matches it, so `tms` must stay off.
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
    // Leaflet renders tooltip content as HTML, and these values are untrusted:
    // "label" is a player name taken from the mission, so a player called
    // "<img src=x onerror=...>" would otherwise execute in every viewer's
    // browser. Everything interpolated here must be escaped.
    const label = u.label || u.type;
    return (
      `<strong>${escapeHTML(label)}</strong><br/>` +
      `${escapeHTML(u.type)}<br/>` +
      `alt ${Math.round(u.alt)} m · cap ${Math.round(u.heading)}°`
    );
  }

  /**
   * Tells Leaflet its container changed size. Needed when the map gains or
   * loses the panels around it: Leaflet caches the size and would otherwise keep
   * drawing at the old dimensions, leaving grey bands.
   */
  export function resize() {
    map?.invalidateSize();
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

  /* Aeronautical style: the relief base is already chart-like, so it is calmed
     only slightly — enough for the callouts to stand out, not enough to wash the
     terrain away (a heavier filter made the map unreadable). */
  :global(.dcsmm-aero-tiles) {
    filter: saturate(0.82) brightness(1.03) contrast(1.04);
  }

  /* Chart-style airfield label: a small boxed callout, as on a paper chart. */
  :global(.dcsmm-chart-label) {
    padding: 0.15rem 0.35rem;
    font-size: 0.68rem;
    line-height: 1.25;
    color: #1b1f24;
    background: rgba(255, 255, 255, 0.9);
    /* A cyan edge ties the label to the airfield marker, which is the only
       other cyan thing on the map. */
    border: 1px solid #6b7280;
    border-left: 3px solid #22d3ee;
    border-radius: 3px;
    box-shadow: 0 1px 3px rgba(0, 0, 0, 0.45);
    white-space: nowrap;
  }

  :global(.dcsmm-chart-label::before) {
    display: none;
  }

  :global(.dcsmm-chart-label .icao) {
    color: #4b5563;
    font-weight: 400;
  }

  :global(.leaflet-container) {
    background: #0b0f14;
    font: inherit;
  }
</style>
