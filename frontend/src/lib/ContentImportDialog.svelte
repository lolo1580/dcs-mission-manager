<script>
  import { createEventDispatcher } from 'svelte';
  import { t } from './i18n.js';
  import { fmtSize } from './install.js';
  import { inspectArchive, commitImport, cancelImport } from './content.js';
  import Modal from './ui/Modal.svelte';
  import Badge from './ui/Badge.svelte';

  export let open = false;

  const dispatch = createEventDispatcher();
  let fileInput;
  let file = null;
  let preview = null;
  let selections = {};
  let replace = false;
  let busy = false;
  let error = '';
  let committed = false;

  $: if (!open) {
    // Keep the state until the modal is closed, then start fresh next time.
    if (!open && preview && !committed) cancelImport(preview.id);
    if (!open) reset();
  }

  function reset() {
    file = null;
    preview = null;
    selections = {};
    replace = false;
    busy = false;
    error = '';
    committed = false;
    if (fileInput) fileInput.value = '';
  }

  function selectAll(value) {
    for (const item of preview?.items ?? []) {
      if (selections[item.id]) selections[item.id].selected = value;
    }
    selections = { ...selections };
  }

  async function pick(event) {
    file = event.currentTarget.files?.[0] ?? null;
    if (!file) return;
    await inspect();
  }

  async function inspect() {
    if (!file) return;
    busy = true;
    error = '';
    try {
      const result = await inspectArchive(file);
      preview = result;
      const next = {};
      for (const item of result.items ?? []) {
        next[item.id] = {
          selected: (item.conflicts?.length ?? 0) === 0,
          category: item.category ?? '',
          aircraft: item.aircraft ?? '',
          name: item.name ?? '',
        };
      }
      selections = next;
    } catch (err) {
      error = err.message;
      if (fileInput) fileInput.value = '';
    } finally {
      busy = false;
    }
  }

  async function commit() {
    if (!preview) return;
    const selected = Object.entries(selections)
      .filter(([, value]) => value.selected)
      .map(([id, value]) => ({
        id,
        category: value.category,
        aircraft: value.aircraft,
        name: value.name,
      }));
    if (selected.length === 0) {
      error = $t('content.import.selectOne');
      return;
    }
    busy = true;
    error = '';
    try {
      await commitImport(preview.id, selected, replace);
      committed = true;
      dispatch('imported');
    } catch (err) {
      error = err.message;
    } finally {
      busy = false;
    }
  }

  function close() {
    if (busy) return;
    open = false;
  }

  function warningLabel(code) {
    return $t(`content.warn.${code}`);
  }

  function setSelected(id, value) {
    if (!selections[id]) return;
    selections[id].selected = value;
    selections = { ...selections };
  }

  function setField(id, key, value) {
    if (!selections[id]) return;
    selections[id][key] = value;
    selections = { ...selections };
  }

  $: selectedCount = Object.values(selections).filter((value) => value.selected).length;
</script>

<Modal {open} title={$t('content.import.title')} subtitle={$t('content.import.subtitle')} wide on:close={close}>
  <div class="import">
    {#if error}<p class="error">{error}</p>{/if}

    {#if !preview}
      <div class="drop">
        <p>{$t('content.import.choose')}</p>
        <p class="muted">{$t('content.import.formats')}</p>
        <input bind:this={fileInput} type="file" accept=".zip,.7z,.rar,application/zip,application/x-7z-compressed,application/vnd.rar" on:change={pick} />
        {#if busy}<p class="busy">{$t('content.import.analyzing')}</p>{/if}
      </div>
      <p class="warning">⚠ {$t('content.import.warning')}</p>
    {:else}
      <div class="preview-head">
        <div>
          <strong>{preview.archiveName}</strong>
          <span class="muted"> · {preview.format.toUpperCase()} · {$t('content.import.found', { count: preview.items.length })}</span>
        </div>
        <div class="select-actions">
          <button class="btn ghost" type="button" on:click={() => selectAll(true)}>{$t('content.import.selectAll')}</button>
          <button class="btn ghost" type="button" on:click={() => selectAll(false)}>{$t('content.import.selectNone')}</button>
        </div>
      </div>

      <div class="items">
        {#each preview.items as item (item.id)}
          <div class="item" class:chosen={selections[item.id]?.selected}>
            <label class="check">
              <input
                type="checkbox"
                checked={selections[item.id]?.selected ?? false}
                on:change={(e) => setSelected(item.id, e.currentTarget.checked)}
              />
            </label>
            <div class="item-body">
              <div class="item-title">
                <Badge tone={item.kind === 'mod' ? 'blue' : 'green'}>
                  {item.kind === 'mod' ? $t('content.kind.mod') : $t('content.kind.livery')}
                </Badge>
                <span class="muted">{fmtSize(item.sizeBytes)} · {$t('content.mods.files', { count: item.files })}</span>
                {#if item.structure}<Badge tone={item.structure === 'standard' ? 'green' : 'amber'}>{$t(`content.livery.${item.structure}`)}</Badge>{/if}
              </div>
              <div class="fields">
                {#if item.kind === 'mod'}
                  <label class="field">
                    <span>{$t('content.mods.category')}</span>
                    <input
                      value={selections[item.id]?.category ?? ''}
                      placeholder="aircraft / tech"
                      on:input={(e) => setField(item.id, 'category', e.currentTarget.value)}
                    />
                  </label>
                {:else}
                  <label class="field">
                    <span>{$t('content.livery.aircraft')}</span>
                    <input
                      value={selections[item.id]?.aircraft ?? ''}
                      placeholder={$t('content.livery.unknownAircraft')}
                      on:input={(e) => setField(item.id, 'aircraft', e.currentTarget.value)}
                    />
                  </label>
                {/if}
                <label class="field name">
                  <span>{$t('content.import.name')}</span>
                  <input
                    value={selections[item.id]?.name ?? ''}
                    on:input={(e) => setField(item.id, 'name', e.currentTarget.value)}
                  />
                </label>
              </div>
              {#if item.targetRel}
                <p class="target">→ <code>{item.targetRel}</code></p>
              {/if}
              {#if item.conflicts?.length}
                <p class="conflict">⚠ {$t('content.import.conflict', { target: item.conflicts.join(', ') })}</p>
              {/if}
              {#if item.warnings?.length}
                <div class="warnings">
                  {#each item.warnings as warning (warning.code + (warning.detail ?? ''))}
                    <span class="warn">⚠ {warningLabel(warning.code)}{#if warning.detail} · {warning.detail}{/if}</span>
                  {/each}
                </div>
              {/if}
            </div>
          </div>
        {/each}
      </div>

      <label class="replace">
        <input type="checkbox" bind:checked={replace} />
        {$t('content.import.replace')}
      </label>
    {/if}
  </div>

  <div slot="footer" class="footer">
    <button class="btn ghost" type="button" on:click={close} disabled={busy}>{$t('content.import.cancel')}</button>
    {#if preview}
      <button class="btn primary" type="button" on:click={commit} disabled={busy || selectedCount === 0}>
        {$t(busy ? 'content.import.installing' : 'content.import.commit', { count: selectedCount })}
      </button>
    {/if}
  </div>
</Modal>

<style>
  .import {
    display: flex;
    flex-direction: column;
    gap: 0.7rem;
    min-width: 0;
  }

  .drop {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.4rem;
    padding: 1.2rem;
    text-align: center;
    border: 1px dashed var(--border-strong);
    border-radius: var(--radius-lg);
    background: var(--bg-elevated);
  }

  .drop p {
    margin: 0;
    font-size: 0.8rem;
  }

  .drop input[type='file'] {
    max-width: 100%;
    margin-top: 0.4rem;
    font-size: 0.78rem;
  }

  .busy,
  .muted {
    color: var(--muted);
    font-size: 0.75rem;
  }

  .warning {
    margin: 0;
    color: var(--amber);
    font-size: 0.75rem;
    line-height: 1.45;
  }

  .preview-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.5rem;
    flex-wrap: wrap;
  }

  .preview-head strong {
    font-size: 0.85rem;
  }

  .select-actions {
    display: flex;
    gap: 0.25rem;
  }

  .items {
    display: flex;
    flex-direction: column;
    gap: 0.45rem;
    max-height: 50vh;
    overflow-y: auto;
    padding-right: 0.15rem;
  }

  .item {
    display: flex;
    gap: 0.55rem;
    padding: 0.55rem 0.65rem;
    background: var(--bg-elevated);
    border: 1px solid var(--border);
    border-radius: var(--radius);
  }

  .item.chosen {
    border-color: color-mix(in srgb, var(--blue) 40%, var(--border));
  }

  .check {
    padding-top: 0.15rem;
  }

  .item-body {
    display: flex;
    flex-direction: column;
    gap: 0.3rem;
    min-width: 0;
    flex: 1;
  }

  .item-title {
    display: flex;
    align-items: center;
    gap: 0.35rem;
    flex-wrap: wrap;
    font-size: 0.75rem;
  }

  .fields {
    display: flex;
    gap: 0.4rem;
    flex-wrap: wrap;
  }

  .field {
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
    min-width: 110px;
  }

  .field.name {
    flex: 1;
    min-width: 160px;
  }

  .field span {
    font-size: 0.62rem;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    color: var(--faint);
  }

  .field input {
    width: 100%;
    padding: 0.32rem 0.42rem;
    color: var(--text);
    background: var(--bg);
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-sm);
    font-size: 0.78rem;
  }

  .target,
  .conflict {
    margin: 0;
    color: var(--muted);
    font-size: 0.68rem;
  }

  .target code {
    color: var(--text);
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  }

  .conflict {
    color: var(--amber);
  }

  .warnings {
    display: flex;
    flex-direction: column;
    gap: 0.1rem;
  }

  .warn {
    color: var(--amber);
    font-size: 0.68rem;
  }

  .replace {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    font-size: 0.76rem;
    color: var(--muted);
  }

  .footer {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 0.4rem;
    width: 100%;
  }

  .error {
    margin: 0;
    padding: 0.45rem 0.55rem;
    color: var(--red);
    background: var(--red-soft);
    border: 1px solid color-mix(in srgb, var(--red) 40%, transparent);
    border-radius: var(--radius);
    font-size: 0.76rem;
  }
</style>
