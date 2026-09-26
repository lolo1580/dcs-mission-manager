<script>
  import { onDestroy } from 'svelte';
  import { viewingChart, chartURL } from './aerodromes.js';
  import { t } from './i18n.js';

  // Escape closes the viewer; the overlay is a modal, so the keyboard shortcut is
  // expected behaviour rather than a nicety.
  function onKey(e) {
    if (e.key === 'Escape') viewingChart.set(null);
  }
  if (typeof window !== 'undefined') window.addEventListener('keydown', onKey);
  onDestroy(() => {
    if (typeof window !== 'undefined') window.removeEventListener('keydown', onKey);
  });
</script>

{#if $viewingChart}
  {@const c = $viewingChart}
  <div class="overlay">
    <header>
      <span class="kind">{$t('charts.kind.' + c.kind)}</span>
      <strong>{c.name}</strong>
      <a class="link" href={chartURL(c)} target="_blank" rel="noopener">{$t('charts.openTab')}</a>
      <button class="close" title={$t('charts.close')} on:click={() => viewingChart.set(null)}>×</button>
    </header>
    <!-- A scan is a document: it is displayed whole, never warped onto the map. -->
    <div class="stage">
      <img src={chartURL(c)} alt={c.name} />
    </div>
    <p class="note">{$t('charts.note')}</p>
  </div>
{/if}

<style>
  .overlay {
    position: fixed;
    inset: 0;
    z-index: 2000;
    display: flex;
    flex-direction: column;
    background: rgba(8, 11, 15, 0.94);
  }

  header {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    padding: 0.5rem 0.8rem;
    background: var(--panel);
    border-bottom: 1px solid var(--border);
  }

  header strong {
    flex: 1;
    font-size: 0.85rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .kind {
    padding: 0.1rem 0.4rem;
    font-size: 0.68rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--muted);
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 999px;
  }

  .link {
    font-size: 0.75rem;
    color: var(--blue);
    text-decoration: none;
  }

  .link:hover {
    text-decoration: underline;
  }

  .close {
    padding: 0 0.4rem;
    font-size: 1.2rem;
    line-height: 1;
    color: var(--muted);
    background: transparent;
    border: none;
    cursor: pointer;
  }

  .close:hover {
    color: var(--text);
  }

  .stage {
    flex: 1;
    min-height: 0;
    overflow: auto;
    text-align: center;
  }

  img {
    max-width: 100%;
    height: auto;
    background: #fff;
  }

  .note {
    margin: 0;
    padding: 0.35rem 0.8rem;
    font-size: 0.7rem;
    color: var(--muted);
    background: var(--panel);
    border-top: 1px solid var(--border);
  }
</style>
