<script>
  /**
   * Panels: the Logitech/Saitek flight panels the manager drives directly, and
   * the DCS-BIOS link it listens to. This is the cockpit-hardware side, absorbed
   * from the DCS Panel Manager.
   */
  import { onMount } from 'svelte';
  import {
    panels,
    panelsSupported,
    panelsError,
    panelsLoading,
    biosState,
    panelEvents,
    loadPanels,
    modelLabel,
    fmtIDs,
  } from './panels.js';
  import { t, lang } from './i18n.js';

  onMount(loadPanels);

  const KIND_KEYS = {
    connected: 'panels.kind.connected',
    disconnected: 'panels.kind.disconnected',
    input: 'panels.kind.input',
    error: 'panels.kind.error',
  };

  function kindLabel(kind) {
    return $t(KIND_KEYS[kind] ?? kind);
  }

  /** The panel a device path belongs to, for the event list. */
  function deviceLabel(path) {
    const p = $panels.find((d) => d.Path === path || d.path === path);
    return p ? modelLabel(p.Model ?? p.model) : path;
  }

  function timeOf(ms) {
    return new Date(ms).toLocaleTimeString();
  }
</script>

<section class="panels">
  <header>
    <h2>
      {$t('panels.title')}
      <span class="count">{$panels.length}</span>
    </h2>
    <button class="refresh" on:click={loadPanels} disabled={$panelsLoading}>
      {$t('stats.refresh')}
    </button>
  </header>

  {#if $panelsError}
    <p class="error">{$panelsError}</p>
  {/if}

  {#if !$panelsSupported}
    <p class="empty">{$t('panels.unsupported')}</p>
  {:else}
    <div class="block">
      <h3>
        {$t(($biosState.available ? 'panels.bios' : 'panels.biosUnavailable'))}
        <span class="badge" class:on={$biosState.connected}>
          {$biosState.connected ? $t('panels.connected') : $t('panels.disconnected')}
        </span>
      </h3>
      {#if $biosState.available}
        <p class="line">
          {#if $biosState.aircraft}
            {$t('panels.aircraft')}: <b>{$biosState.aircraft}</b>
          {:else}
            <span class="muted">{$t('panels.noAircraft')}</span>
          {/if}
        </p>
        <p class="line muted">{$biosState.frames} {$t('panels.frames')}</p>
      {:else}
        <p class="line muted">{$t('panels.biosUnavailableNote')}</p>
      {/if}
    </div>

    <div class="block">
      <h3>{$t('panels.devices')}</h3>
      {#if $panels.length === 0}
        <p class="empty">{$t('panels.none')}</p>
      {:else}
        <div class="cards">
          {#each $panels as d (d.Path ?? d.path)}
            <div class="card">
              <div class="row">
                <strong>{modelLabel(d.Model ?? d.model)}</strong>
                <span class="badge on">{$t('panels.connected')}</span>
              </div>
              <dl>
                <div><dt>{$t('panels.ids')}</dt><dd>{fmtIDs(d.VendorID ?? d.vendorId, d.ProductID ?? d.productId)}</dd></div>
                {#if d.Product ?? d.product}
                  <div><dt>{$t('panels.product')}</dt><dd>{d.Product ?? d.product}</dd></div>
                {/if}
                {#if d.Serial ?? d.serial}
                  <div><dt>{$t('panels.serial')}</dt><dd>{d.Serial ?? d.serial}</dd></div>
                {/if}
              </dl>
            </div>
          {/each}
        </div>
      {/if}
    </div>

    <div class="block">
      <h3>
        {$t('panels.monitor')}
        <span class="count">{$panelEvents.length}</span>
      </h3>
      {#if $panelEvents.length === 0}
        <p class="empty">{$t('panels.noEvents')}</p>
      {:else}
        <ul class="events">
          {#each $panelEvents as e (`${e.at}-${e.device}-${e.input?.control ?? e.kind}`)}
            <li class:error={e.kind === 'error'}>
              <span class="t">{timeOf(e.at)}</span>
              <span class="dev">{deviceLabel(e.device)}</span>
              {#if e.kind === 'input'}
                <span class="ctl">{e.input.control}</span>
                <span class="state" class:on={e.input.active}>
                  {e.input.active ? $t('panels.on') : $t('panels.off')}
                </span>
              {:else if e.kind === 'error'}
                <span class="msg">{e.error}</span>
              {:else}
                <span class="msg">{kindLabel(e.kind)}</span>
              {/if}
            </li>
          {/each}
        </ul>
      {/if}
    </div>
  {/if}
</section>

<style>
  .panels {
    display: flex;
    flex-direction: column;
    min-height: 0;
    height: 100%;
    padding: 0.9rem 1rem;
    overflow-y: auto;
  }

  header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.5rem;
    margin-bottom: 0.6rem;
  }

  h2 {
    margin: 0;
    font-size: 0.8rem;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--muted);
    display: flex;
    gap: 0.5rem;
    align-items: baseline;
  }

  .count {
    color: var(--text);
    font-weight: 400;
    font-variant-numeric: tabular-nums;
    text-transform: none;
    letter-spacing: 0;
    font-size: 0.76rem;
  }

  .refresh {
    padding: 0.25rem 0.55rem;
    font-size: 0.74rem;
    color: var(--muted);
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 6px;
    cursor: pointer;
  }

  .refresh:hover:not(:disabled) {
    color: var(--text);
    border-color: var(--blue);
  }

  .block {
    margin-bottom: 1.1rem;
  }

  h3 {
    margin: 0 0 0.4rem;
    font-size: 0.78rem;
    color: var(--text);
    display: flex;
    gap: 0.5rem;
    align-items: center;
  }

  .line {
    margin: 0.2rem 0;
    font-size: 0.78rem;
    color: var(--text);
  }

  .line.muted {
    color: var(--muted);
  }

  .badge {
    padding: 0.05rem 0.4rem;
    border-radius: 999px;
    font-size: 0.66rem;
    color: var(--muted);
    border: 1px solid var(--border);
  }

  .badge.on {
    color: var(--green);
    border-color: color-mix(in srgb, var(--green) 45%, var(--border));
  }

  .cards {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
    gap: 0.5rem;
    max-width: 820px;
  }

  .card {
    padding: 0.5rem 0.6rem;
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 8px;
  }

  .row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.5rem;
    margin-bottom: 0.35rem;
  }

  .row strong {
    font-size: 0.82rem;
  }

  dl {
    margin: 0;
    display: grid;
    gap: 0.15rem;
    font-size: 0.74rem;
  }

  dl div {
    display: flex;
    justify-content: space-between;
    gap: 0.75rem;
  }

  dt {
    color: var(--muted);
  }

  dd {
    margin: 0;
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    font-size: 0.72rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .events {
    list-style: none;
    margin: 0;
    padding: 0;
    font-size: 0.76rem;
    max-width: 820px;
    max-height: 320px;
    overflow-y: auto;
  }

  .events li {
    display: flex;
    gap: 0.5rem;
    padding: 0.15rem 0.3rem;
    border-top: 1px solid var(--border);
  }

  .events li:first-child {
    border-top: none;
  }

  .events li.error {
    color: #f0b429;
  }

  .t {
    color: var(--muted);
    font-variant-numeric: tabular-nums;
    flex: none;
  }

  .dev {
    color: var(--blue);
    flex: none;
    max-width: 160px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .ctl {
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    font-size: 0.72rem;
  }

  .state {
    color: var(--muted);
    flex: none;
  }

  .state.on {
    color: var(--green);
  }

  .msg {
    color: var(--muted);
    overflow-wrap: anywhere;
  }

  .empty {
    color: var(--muted);
    font-size: 0.8rem;
    line-height: 1.5;
  }

  .error {
    color: #f0b429;
    font-size: 0.78rem;
  }
</style>
