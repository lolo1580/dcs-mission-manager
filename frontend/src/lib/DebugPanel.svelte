<script>
  /**
   * Debug: the runtime debug switch and the live log. It is its own sub-tab of
   * Settings, so it can be left open (the log updates over SSE) while the user
   * drives the panels.
   */
  import { onMount } from 'svelte';
  import {
    debugEnabled,
    debugAvailable,
    debugLines,
    hasDebugLines,
    loadDebug,
    setDebug,
    clearDebug,
  } from './debug.js';
  import { t } from './i18n.js';

  onMount(loadDebug);

  function timeOf(ms) {
    return new Date(ms).toLocaleTimeString();
  }

  // The area of a line, as a short stable label, so the viewer can colour it.
  function areaLabel(area) {
    return area || 'app';
  }
</script>

<section class="debug">
  <header>
    <h2>{$t('debug.title')}</h2>
    <div class="actions">
      <button class="refresh" on:click={loadDebug}>{$t('stats.refresh')}</button>
      <button class="refresh" on:click={clearDebug} disabled={!$hasDebugLines}>{$t('debug.clear')}</button>
    </div>
  </header>

  {#if !$debugAvailable}
    <p class="empty">{$t('debug.unavailable')}</p>
  {:else}
    <label class="toggle">
      <input type="checkbox" checked={$debugEnabled} on:change={(e) => setDebug(e.currentTarget.checked)} />
      {$t('debug.enable')}
    </label>
    <p class="hint">{$t('debug.hint')}</p>

    <div class="log" aria-live="polite">
      {#if !$hasDebugLines}
        <p class="empty">{$debugEnabled ? $t('debug.noLines') : $t('debug.off')}</p>
      {:else}
        {#each $debugLines as line}
          <div class="line" class:error={line.level === 'error'} class:warn={line.level === 'warn'}>
            <span class="t">{timeOf(line.at)}</span>
            <span class="area">{areaLabel(line.area)}</span>
            <span class="msg">{line.message}</span>
          </div>
        {/each}
      {/if}
    </div>
  {/if}
</section>

<style>
  .debug {
    display: flex;
    flex-direction: column;
    min-height: 0;
    height: 100%;
    padding: 0.9rem 1rem;
    overflow: hidden;
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
  }

  .actions {
    display: flex;
    gap: 0.35rem;
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

  .refresh:disabled {
    opacity: 0.5;
    cursor: default;
  }

  .toggle {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    font-size: 0.78rem;
    color: var(--text);
    cursor: pointer;
  }

  .hint {
    margin: 0.25rem 0 0.6rem;
    font-size: 0.72rem;
    color: var(--muted);
    max-width: 80ch;
    line-height: 1.5;
  }

  .log {
    flex: 1 1 auto;
    min-height: 0;
    overflow-y: auto;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--bg);
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    font-size: 0.72rem;
  }

  .line {
    display: flex;
    gap: 0.6rem;
    padding: 0.16rem 0.5rem;
    border-top: 1px solid var(--border);
  }

  .line:first-child {
    border-top: none;
  }

  .line.warn {
    color: #f0b429;
  }

  .line.error {
    color: var(--red);
  }

  .t {
    color: var(--muted);
    flex: none;
    font-variant-numeric: tabular-nums;
  }

  .area {
    flex: none;
    width: 5.5rem;
    color: var(--blue);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .msg {
    overflow-wrap: anywhere;
  }

  .empty {
    color: var(--muted);
    font-size: 0.8rem;
    line-height: 1.5;
    padding: 0.4rem 0.5rem;
  }
</style>
