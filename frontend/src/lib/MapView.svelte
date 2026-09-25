<script>
  import { onMount, onDestroy } from 'svelte';
  import L from 'leaflet';
  import 'leaflet/dist/leaflet.css';

  import {
    visibleUnits,
    filters,
    connect,
    connected,
    lastUpdate,
    selectedId,
    selectedUnit,
    units,
    fetchTheatres,
  } from './units.js';
  import { unitIcon, coalitionColor } from './icons.js';

  let mapEl;
  let map;
  /** @type {Map<string, {marker: any, trail: any, trailPts: Array<[number,number]>}>} */
  const layers = new Map();
  let trailLayer;
  let userMoved = false;
  let followOwnship = true;

  const TRAIL_MAX = 120;

  onMount(async () => {
    map = L.map(mapEl, { zoomControl: true, preferCanvas: true }).setView([45, 40], 5);
    trailLayer = L.layerGroup().addTo(map);

    // Basemap: prefer authentic DCS tiles when available, otherwise a real map.
    try {
      const meta = await fetchTheatres();
      configureBasemap(meta);
    } catch {
      L.tileLayer('https://tile.openstreetmap.org/{z}/{x}/{y}.png', {
        attribution: '© OpenStreetMap',
        maxZoom: 12,
      }).addTo(map);
    }

    // Stop auto-following as soon as the user pans or zooms manually.
    map.on('dragstart', () => (userMoved = true));
    map.on('zoomstart', (e) => {
      if (e.originalEvent) userMoved = true;
    });

    const stop = connect();
    const unsub = visibleUnits.subscribe(syncMarkers);

    return () => {
      unsub?.();
      stop?.();
      map?.remove();
    };
  });

  let basemap = null;

  function configureBasemap(meta) {
    const th = meta.theatres?.find((t) => t.id === meta.default) ?? meta.theatres?.[0];
    if (th?.tiles) {
      basemap = L.tileLayer(`/api/tiles/${th.id}/{z}/{x}/{y}.png`, {
        attribution: 'DCS World',
        minZoom: 0,
        maxZoom: 8,
        tms: false,
        errorTileUrl: '',
      }).addTo(map);
      map.setView([(th.bounds.minLat + th.bounds.maxLat) / 2, (th.bounds.minLng + th.bounds.maxLng) / 2], 6);
    } else {
      basemap = L.tileLayer(meta.basemap || 'https://tile.openstreetmap.org/{z}/{x}/{y}.png', {
        attribution: '© OpenStreetMap',
        maxZoom: 12,
      }).addTo(map);
      if (th) {
        const b = th.bounds;
        map.fitBounds([
          [b.minLat, b.minLng],
          [b.maxLat, b.maxLng],
        ]);
      }
    }
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
    }
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

  :global(.leaflet-container) {
    background: #0b0f14;
    font: inherit;
  }
</style>
