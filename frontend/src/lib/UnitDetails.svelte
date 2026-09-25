<script>
  import { selectedUnit, selectedId } from './units.js';
  import { coalitionColor, CATEGORY_LABELS, COALITION_LABELS } from './icons.js';

  function fmt(v, digits = 0) {
    return typeof v === 'number' ? v.toLocaleString(undefined, { maximumFractionDigits: digits }) : '—';
  }
</script>

{#if $selectedUnit}
  {@const u = $selectedUnit}
  <div class="card">
    <header>
      <span class="dot" style="background:{coalitionColor(u.coalition)}"></span>
      <strong>{u.label || u.type}</strong>
      <button class="close" title="Fermer" on:click={() => selectedId.set(null)}>×</button>
    </header>

    <dl>
      <div><dt>Type</dt><dd>{u.type}</dd></div>
      <div><dt>Catégorie</dt><dd>{CATEGORY_LABELS[u.category] ?? u.category}</dd></div>
      <div><dt>Coalition</dt><dd>{COALITION_LABELS[u.coalition] ?? u.coalition}</dd></div>
      {#if u.country}<div><dt>Pays</dt><dd>{u.country}</dd></div>{/if}
      <div><dt>Latitude</dt><dd>{fmt(u.lat, 5)}</dd></div>
      <div><dt>Longitude</dt><dd>{fmt(u.lng, 5)}</dd></div>
      <div><dt>Altitude</dt><dd>{fmt(u.alt)} m</dd></div>
      <div><dt>Cap</dt><dd>{fmt(u.heading)}°</dd></div>
      <div><dt>Mise à jour</dt><dd>{u.ageMs} ms</dd></div>
      {#if u.ownship}<div><dt>Rôle</dt><dd>Mon appareil</dd></div>{/if}
    </dl>
  </div>
{/if}

<style>
  .card {
    position: absolute;
    right: 1rem;
    bottom: 1rem;
    z-index: 1000;
    width: 260px;
    padding: 0.8rem 0.9rem;
    background: var(--panel);
    border: 1px solid var(--border);
    border-radius: 10px;
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.4);
  }

  header {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    margin-bottom: 0.6rem;
  }

  header strong {
    flex: 1;
    font-size: 0.9rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .dot {
    width: 9px;
    height: 9px;
    border-radius: 50%;
    flex: none;
  }

  .close {
    background: transparent;
    border: none;
    color: var(--muted);
    font-size: 1.1rem;
    line-height: 1;
    cursor: pointer;
  }

  .close:hover {
    color: var(--text);
  }

  dl {
    margin: 0;
    display: grid;
    gap: 0.25rem;
    font-size: 0.8rem;
  }

  dl div {
    display: flex;
    justify-content: space-between;
    gap: 1rem;
  }

  dt {
    color: var(--muted);
  }

  dd {
    margin: 0;
    font-variant-numeric: tabular-nums;
    text-align: right;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
