<script>
  import {
    filters,
    resetFilters,
    summary,
    visibleUnits,
    units,
    selectedId,
    CATEGORIES,
    COALITIONS,
  } from './units.js';
  import { coalitionColor, CATEGORY_LABELS } from './icons.js';

  function toggle(list, id) {
    return list.includes(id) ? list.filter((x) => x !== id) : [...list, id];
  }

  function toggleCategory(id) {
    filters.update((f) => ({ ...f, categories: toggle(f.categories, id) }));
  }

  function toggleCoalition(id) {
    filters.update((f) => ({ ...f, coalitions: toggle(f.coalitions, id) }));
  }
</script>

<aside>
  <section>
    <h2>Filtres</h2>
    <input
      class="search"
      type="search"
      placeholder="Rechercher (type, nom, pays)…"
      value={$filters.search}
      on:input={(e) => filters.update((f) => ({ ...f, search: e.currentTarget.value }))}
    />

    <div class="chips" role="group" aria-label="Coalitions">
      {#each COALITIONS as c}
        <button
          class="chip"
          class:active={$filters.coalitions.includes(c.id)}
          style="--c:{coalitionColor(c.id)}"
          on:click={() => toggleCoalition(c.id)}
        >
          <span class="swatch"></span>{c.label}
          <span class="n">{$summary.byCoalition?.[c.id] ?? 0}</span>
        </button>
      {/each}
    </div>

    <div class="chips" role="group" aria-label="Catégories">
      {#each CATEGORIES as c}
        <button
          class="chip"
          class:active={$filters.categories.includes(c.id)}
          on:click={() => toggleCategory(c.id)}
        >
          {c.label}
          <span class="n">{$summary.byCategory?.[c.id] ?? 0}</span>
        </button>
      {/each}
    </div>

    <label class="check">
      <input
        type="checkbox"
        checked={$filters.ownshipOnly}
        on:change={(e) => filters.update((f) => ({ ...f, ownshipOnly: e.currentTarget.checked }))}
      />
      Mon appareil uniquement
    </label>

    <label class="check">
      <input
        type="checkbox"
        checked={$filters.showTrails}
        on:change={(e) => filters.update((f) => ({ ...f, showTrails: e.currentTarget.checked }))}
      />
      Afficher les traces de vol
    </label>

    <button class="reset" on:click={resetFilters}>Réinitialiser les filtres</button>
  </section>

  <section class="list-section">
    <h2>
      Unités <span class="count">{$visibleUnits.length} / {$units.length}</span>
    </h2>
    <ul>
      {#each $visibleUnits.slice(0, 300) as u (u.id)}
        <li>
          <button class:selected={$selectedId === u.id} on:click={() => selectedId.set(u.id)}>
            <span class="dot" style="background:{coalitionColor(u.coalition)}"></span>
            <span class="type">{u.label || u.type}</span>
            <span class="cat">{CATEGORY_LABELS[u.category] ?? u.category}</span>
          </button>
        </li>
      {/each}
      {#if $visibleUnits.length > 300}
        <li class="more">+ {$visibleUnits.length - 300} autres…</li>
      {/if}
      {#if $visibleUnits.length === 0}
        <li class="empty">Aucune unité</li>
      {/if}
    </ul>
  </section>
</aside>

<style>
  aside {
    display: flex;
    flex-direction: column;
    width: 320px;
    min-width: 320px;
    height: 100%;
    overflow: hidden;
    background: var(--panel);
    border-right: 1px solid var(--border);
  }

  section {
    padding: 0.9rem 1rem;
    border-bottom: 1px solid var(--border);
  }

  .list-section {
    display: flex;
    flex-direction: column;
    flex: 1;
    min-height: 0;
    border-bottom: none;
  }

  h2 {
    margin: 0 0 0.7rem;
    font-size: 0.8rem;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--muted);
    display: flex;
    justify-content: space-between;
  }

  .count {
    color: var(--text);
    font-variant-numeric: tabular-nums;
  }

  .search {
    width: 100%;
    padding: 0.45rem 0.6rem;
    margin-bottom: 0.6rem;
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 6px;
    color: var(--text);
    font-size: 0.85rem;
  }

  .search:focus {
    outline: none;
    border-color: var(--blue);
  }

  .chips {
    display: flex;
    flex-wrap: wrap;
    gap: 0.35rem;
    margin-bottom: 0.6rem;
  }

  .chip {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    padding: 0.25rem 0.5rem;
    font-size: 0.78rem;
    color: var(--muted);
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 999px;
    cursor: pointer;
  }

  .chip:hover {
    color: var(--text);
  }

  .chip.active {
    color: var(--text);
    border-color: var(--c, var(--blue));
    background: color-mix(in srgb, var(--c, var(--blue)) 18%, var(--bg));
  }

  .swatch {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--c, var(--muted));
  }

  .chip .n {
    opacity: 0.6;
    font-variant-numeric: tabular-nums;
  }

  .check {
    display: flex;
    align-items: center;
    gap: 0.45rem;
    font-size: 0.82rem;
    color: var(--muted);
    margin-bottom: 0.4rem;
    cursor: pointer;
  }

  .reset {
    margin-top: 0.3rem;
    padding: 0.35rem 0.6rem;
    font-size: 0.78rem;
    color: var(--muted);
    background: transparent;
    border: 1px solid var(--border);
    border-radius: 6px;
    cursor: pointer;
  }

  .reset:hover {
    color: var(--text);
    border-color: var(--muted);
  }

  ul {
    list-style: none;
    margin: 0;
    padding: 0;
    overflow-y: auto;
    flex: 1;
  }

  li button {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    width: 100%;
    padding: 0.35rem 0.4rem;
    background: transparent;
    border: none;
    border-radius: 6px;
    color: var(--text);
    font-size: 0.82rem;
    text-align: left;
    cursor: pointer;
  }

  li button:hover {
    background: var(--bg);
  }

  li button.selected {
    background: color-mix(in srgb, var(--blue) 22%, var(--bg));
  }

  .dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    flex: none;
  }

  .type {
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .cat {
    color: var(--muted);
    font-size: 0.72rem;
    flex: none;
  }

  .more,
  .empty {
    padding: 0.5rem 0.4rem;
    color: var(--muted);
    font-size: 0.78rem;
  }
</style>
