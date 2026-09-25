<script>
  import { onMount, onDestroy } from 'svelte';
  import L from 'leaflet';
  import 'leaflet/dist/leaflet.css';

  let mapEl;
  let map;
  /** @type {Record<string, L.Marker>} */
  let markers = {};
  let connected = false;
  let unitCount = 0;
  let lastUpdate = null;
  let source;

  const COALITION_COLOR = {
    blue: '#58a6ff',
    red: '#f85149',
    neutral: '#9da7b3',
  };

  function markerColor(unit) {
    return COALITION_COLOR[unit.coalition] ?? COALITION_COLOR.neutral;
  }

  function upsertUnit(unit) {
    if (typeof unit.lat !== 'number' || typeof unit.lng !== 'number') return;
    const latlng = [unit.lat, unit.lng];
    const color = markerColor(unit);

    let marker = markers[unit.name];
    if (!marker) {
      marker = L.circleMarker(latlng, {
        radius: 6,
        color,
        weight: 2,
        fillColor: color,
        fillOpacity: 0.7,
      }).addTo(map);
      marker.bindTooltip(unit.name);
      markers[unit.name] = marker;
    } else {
      marker.setLatLng(latlng);
      marker.setStyle({ color, fillColor: color });
    }
    marker.setTooltipContent(
      `${unit.name}<br/>${unit.unitType || ''}<br/>` +
        `alt ${Math.round(unit.alt)} m · cap ${Math.round(unit.heading)}°`
    );
  }

  function applyState(units) {
    unitCount = units.length;
    lastUpdate = new Date();
    for (const u of units) upsertUnit(u);
  }

  function connect() {
    source = new EventSource('/api/events');
    source.onopen = () => (connected = true);
    source.onerror = () => (connected = false);
    source.onmessage = (e) => {
      try {
        const msg = JSON.parse(e.data);
        if (msg.type === 'state') applyState(msg.units ?? []);
      } catch {
        /* ignore malformed frames */
      }
    };
  }

  onMount(() => {
    map = L.map(mapEl, { zoomControl: true }).setView([45, 40], 5);
    L.tileLayer('https://tile.openstreetmap.org/{z}/{x}/{y}.png', {
      attribution: '© OpenStreetMap',
      maxZoom: 12,
    }).addTo(map);
    connect();

    return () => {
      source?.close();
      map?.remove();
    };
  });

  onDestroy(() => source?.close());
</script>

<div class="layout">
  <header>
    <strong>DCS Mission Manager</strong>
    <span class="live">
      <span class="dot" class:on={connected}></span>
      {connected ? 'connecté' : 'hors ligne'}
    </span>
    <span class="meta">{unitCount} unité{unitCount === 1 ? '' : 's'}</span>
    {#if lastUpdate}
      <span class="meta">maj {lastUpdate.toLocaleTimeString()}</span>
    {/if}
  </header>
  <div class="map" bind:this={mapEl}></div>
</div>

<style>
  .layout {
    display: grid;
    grid-template-rows: auto 1fr;
    height: 100%;
  }

  header {
    display: flex;
    align-items: center;
    gap: 1rem;
    padding: 0.65rem 1rem;
    background: var(--panel);
    border-bottom: 1px solid var(--border);
    font-size: 0.9rem;
  }

  .live {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    color: var(--muted);
  }

  .dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--red);
  }

  .dot.on {
    background: var(--green);
  }

  .meta {
    color: var(--muted);
    margin-left: auto;
  }

  .meta + .meta {
    margin-left: 0;
  }

  .map {
    width: 100%;
    height: 100%;
    background: #0b0f14;
  }
</style>
