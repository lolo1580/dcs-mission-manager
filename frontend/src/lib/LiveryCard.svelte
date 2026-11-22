<script>
  import { createEventDispatcher } from 'svelte';
  import { t } from './i18n.js';
  import { thumbnailURL } from './content.js';
  import { fmtSize } from './install.js';
  import Badge from './ui/Badge.svelte';

  export let livery;
  export let busy = false;
  export let dcsRunning = false;

  const dispatch = createEventDispatcher();
  let imageOK = Boolean(livery.thumbnail);

  function warningLabel(code) {
    return $t(`content.warn.${code}`);
  }

  function initials() {
    const text = (livery.name || '?').trim();
    return text.slice(0, 2).toUpperCase();
  }
</script>

<article class="livery-card">
  <div class="preview">
    {#if livery.thumbnail && imageOK}
      <img src={thumbnailURL(livery.id)} alt="" loading="lazy" on:error={() => (imageOK = false)} />
    {:else}
      <span class="placeholder">{initials()}</span>
    {/if}
    <div class="preview-badges">
      {#if livery.source === 'mod'}
        <Badge>{livery.modName}</Badge>
      {:else if livery.structure === 'flat'}
        <Badge tone="amber">{$t('content.livery.flat')}</Badge>
      {:else if livery.structure === 'misplaced'}
        <Badge tone="red">{$t('content.livery.misplaced')}</Badge>
      {:else if livery.structure === 'nonstandard'}
        <Badge tone="amber">{$t('content.livery.nonstandard')}</Badge>
      {:else}
        <Badge tone="green">{$t('content.livery.standard')}</Badge>
      {/if}
    </div>
  </div>

  <div class="body">
    <div class="title">
      <strong title={livery.name}>{livery.name}</strong>
      {#if livery.aircraft}
        <Badge tone="blue">{livery.aircraft}</Badge>
      {:else}
        <Badge tone="amber">{$t('content.livery.unknownAircraft')}</Badge>
      {/if}
    </div>

    <div class="meta">
      <span>{fmtSize(livery.sizeBytes)}</span>
      <span>·</span>
      <span>{$t('content.mods.files', { count: livery.files })}</span>
      {#if livery.countries?.length}
        <span>·</span>
        <span>{livery.countries.slice(0, 3).join(', ')}</span>
      {/if}
    </div>

    {#if livery.dependencies?.length}
      <p class="deps" title={livery.dependencies.join(', ')}>
        {$t('content.livery.depends')}: {livery.dependencies.slice(0, 3).join(', ')}
      </p>
    {/if}

    {#if livery.warnings?.length}
      <div class="warnings">
        {#each livery.warnings as warning (warning.code + (warning.detail ?? ''))}
          <span class="warn" class:critical={warning.code === 'missing_dependency'}>
            ⚠ {warningLabel(warning.code)}{#if warning.detail}<span class="detail"> · {warning.detail}</span>{/if}
          </span>
        {/each}
      </div>
    {/if}

    {#if livery.source === 'mod'}
      <p class="source">{$t('content.livery.inMod', { mod: livery.modName })}</p>
    {/if}
  </div>

  <div class="actions">
    <button class="btn" type="button" on:click={() => dispatch('reveal', livery)} title={$t('content.livery.reveal')}>📂</button>
    {#if livery.source === 'user'}
      {#if livery.structure === 'flat' || livery.structure === 'misplaced' || livery.structure === 'nonstandard'}
        <button class="btn" type="button" on:click={() => dispatch('classify', livery)} disabled={busy || dcsRunning}>
          {$t('content.livery.classify')}
        </button>
      {/if}
      <button class="btn danger" type="button" on:click={() => dispatch('quarantine', livery)} disabled={busy || dcsRunning}>
        {$t('content.livery.remove')}
      </button>
    {/if}
  </div>
</article>

<style>
  .livery-card {
    display: flex;
    flex-direction: column;
    min-width: 0;
    background: var(--panel);
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    box-shadow: var(--shadow-soft);
    overflow: hidden;
  }

  .preview {
    position: relative;
    display: grid;
    place-items: center;
    aspect-ratio: 16 / 9;
    background:
      radial-gradient(circle at 25% 20%, rgba(76, 154, 255, 0.16), transparent 55%),
      radial-gradient(circle at 75% 80%, rgba(63, 208, 122, 0.1), transparent 50%),
      var(--bg-elevated);
    border-bottom: 1px solid var(--border);
    overflow: hidden;
  }

  .preview img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }

  .placeholder {
    display: grid;
    place-items: center;
    width: 48px;
    height: 48px;
    border-radius: 50%;
    color: var(--blue);
    background: var(--blue-soft);
    border: 1px solid color-mix(in srgb, var(--blue) 35%, var(--border));
    font-weight: 700;
    letter-spacing: 0.04em;
  }

  .preview-badges {
    position: absolute;
    top: 0.4rem;
    left: 0.4rem;
    display: flex;
    gap: 0.25rem;
    max-width: calc(100% - 0.8rem);
  }

  .body {
    display: flex;
    flex-direction: column;
    gap: 0.28rem;
    padding: 0.6rem 0.7rem 0.3rem;
    min-width: 0;
  }

  .title {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 0.35rem;
    min-width: 0;
  }

  .title strong {
    font-size: 0.84rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .meta,
  .deps,
  .source {
    margin: 0;
    color: var(--muted);
    font-size: 0.7rem;
    line-height: 1.4;
  }

  .deps {
    color: var(--faint);
  }

  .source {
    color: var(--blue);
  }

  .warnings {
    display: flex;
    flex-direction: column;
    gap: 0.1rem;
  }

  .warn {
    color: var(--amber);
    font-size: 0.68rem;
    line-height: 1.35;
  }

  .warn.critical {
    color: var(--red);
  }

  .detail {
    color: var(--faint);
  }

  .actions {
    display: flex;
    align-items: center;
    gap: 0.3rem;
    flex-wrap: wrap;
    padding: 0.45rem 0.7rem 0.65rem;
    margin-top: auto;
  }
</style>
