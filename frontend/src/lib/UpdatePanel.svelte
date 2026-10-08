<script>
  import { updateInfo, updateLoading, checkForUpdates } from './updates.js';
  import { t } from './i18n.js';
</script>

<div class="block update-check">
  <div class="heading">
    <h3>{$t('update.title')}</h3>
    <button on:click={() => checkForUpdates(true)} disabled={$updateLoading}>
      {$t($updateLoading ? 'update.checking' : 'update.check')}
    </button>
  </div>
  {#if $updateInfo}
    <p class="line">{$t('update.installed')}: {$updateInfo.currentVersion || '—'}</p>
    {#if $updateInfo.status === 'available'}
      <p class="attention">{$t('update.available')}: {$updateInfo.latestVersion}</p>
      <a href={$updateInfo.downloadUrl} target="_blank" rel="noopener noreferrer">{$t('update.download')}</a>
    {:else if $updateInfo.status === 'current'}
      <p class="line">{$t('update.current')}</p>
    {:else if $updateInfo.status === 'development'}
      <p class="line muted">{$t('update.development')}</p>
    {:else if $updateInfo.status === 'unavailable'}
      <p class="line muted">{$t('update.unavailable')}</p>
    {:else if $updateInfo.status === 'error'}
      <p class="line muted">{$t('update.error')}</p>
    {/if}
  {:else if $updateLoading}
    <p class="line muted">{$t('update.checking')}</p>
  {/if}
</div>

<style>
  .update-check { margin-bottom: 1rem; }
  .heading { display: flex; align-items: center; justify-content: space-between; gap: 1rem; }
  h3 { margin: 0; }
  .line { margin: 0.5rem 0 0; }
  .attention { color: var(--blue); margin: 0.5rem 0; }
  a { color: var(--blue); }
  button {
    padding: 0.25rem 0.55rem;
    font-size: 0.74rem;
    color: var(--muted);
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 6px;
    cursor: pointer;
  }
  button:hover:not(:disabled) { color: var(--text); border-color: var(--blue); }
</style>
