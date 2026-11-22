<script>
  import { onMount } from 'svelte';
  import { t, lang } from './i18n.js';
  import { sessionUser, isServer } from './session.js';
  import { PHASES, builtinTemplates } from './templates.js';
  import {
    checklists,
    checklistsLoading,
    checklistsError,
    checklistsEnabled,
    loadChecklists,
    saveChecklist,
    deleteChecklist,
    loadProgress,
    saveProgress,
    clearProgress,
    newDraft,
    draftFrom,
    parseImport,
    toCSV,
    toJSON,
    downloadText,
  } from './checklists.js';
  import { showToast } from './toast.js';
  import Card from './ui/Card.svelte';
  import Badge from './ui/Badge.svelte';
  import EmptyState from './ui/EmptyState.svelte';
  import Modal from './ui/Modal.svelte';
  import Skeleton from './ui/Skeleton.svelte';

  let query = '';
  let phaseFilter = '';
  let aircraftFilter = '';

  let editorOpen = false;
  let draft = null;
  let editError = '';
  let saving = false;

  let importInput;
  let running = null;
  let progress = { checked: [], current: 0 };
  let activeItem = 0;

  onMount(loadChecklists);

  $: aircrafts = [...new Set($checklists.map((item) => item.aircraft).filter(Boolean))].sort();
  $: templates = builtinTemplates($lang);
  $: visible = $checklists.filter((item) => {
    const text = `${item.name} ${item.aircraft ?? ''} ${item.description ?? ''}`.toLowerCase();
    if (query && !text.includes(query.trim().toLowerCase())) return false;
    if (phaseFilter && item.phase !== phaseFilter) return false;
    if (aircraftFilter && item.aircraft !== aircraftFilter) return false;
    return true;
  });

  $: totalItems = running?.items.length ?? 0;
  $: doneCount = running ? running.items.filter((item) => progress.checked.includes(item.uid)).length : 0;
  $: allDone = totalItems > 0 && doneCount === totalItems;

  function canEditChecklist(checklist) {
    if (!$isServer || !checklist.ownerId) return true;
    return checklist.ownerId === $sessionUser?.id || $sessionUser?.role === 'admin';
  }

  function templateCount() {
    return templates.length;
  }

  // ---- Library ------------------------------------------------------------

  function startNew() {
    draft = newDraft();
    editError = '';
    editorOpen = true;
  }

  function edit(checklist) {
    draft = {
      ...checklist,
      items: checklist.items.map((item) => ({ ...item })),
    };
    editError = '';
    editorOpen = true;
  }

  function closeEditor() {
    if (saving) return;
    editorOpen = false;
    draft = null;
    editError = '';
  }

  function addItem() {
    draft.items = [...draft.items, { uid: '', text: '', response: '', note: '', critical: false }];
  }

  function removeItem(index) {
    draft.items = draft.items.filter((_, i) => i !== index);
    if (draft.items.length === 0) addItem();
  }

  function moveItem(index, delta) {
    const target = index + delta;
    if (target < 0 || target >= draft.items.length) return;
    const items = draft.items.slice();
    [items[index], items[target]] = [items[target], items[index]];
    draft.items = items;
  }

  async function save() {
    editError = '';
    if (!draft.name.trim()) {
      editError = $t('checklist.nameRequired');
      return;
    }
    const items = draft.items
      .map((item) => ({
        uid: item.uid ?? '',
        text: item.text.trim(),
        response: item.response?.trim() ?? '',
        note: item.note?.trim() ?? '',
        critical: Boolean(item.critical),
      }))
      .filter((item) => item.text);
    if (items.length === 0) {
      editError = $t('checklist.itemsRequired');
      return;
    }
    saving = true;
    try {
      await saveChecklist({ ...draft, items });
      editorOpen = false;
      draft = null;
      showToast($t('checklist.saved'), 'success');
    } catch (error) {
      editError = $t('checklist.saveError', { detail: error.message });
    } finally {
      saving = false;
    }
  }

  async function duplicate(checklist) {
    const copy = draftFrom(checklist, { source: checklist.source === 'builtin' ? 'builtin' : 'user' });
    copy.name = `${checklist.name} (copy)`;
    try {
      await saveChecklist(copy);
      showToast($t('checklist.saved'), 'success');
    } catch (error) {
      showToast($t('checklist.saveError', { detail: error.message }), 'error');
    }
  }

  async function useTemplate(template) {
    const copy = draftFrom(
      { ...template, visibility: 'private' },
      { source: 'builtin', name: template.name },
    );
    try {
      const saved = await saveChecklist(copy);
      showToast($t('checklist.saved'), 'success');
      run(saved);
    } catch (error) {
      showToast($t('checklist.saveError', { detail: error.message }), 'error');
    }
  }

  async function remove(checklist) {
    if (!confirm($t('checklist.confirmDelete', { name: checklist.name }))) return;
    try {
      await deleteChecklist(checklist.uid);
      if (running?.uid === checklist.uid) running = null;
      showToast($t('checklist.deleted'), 'success');
    } catch (error) {
      showToast($t('checklist.saveError', { detail: error.message }), 'error');
    }
  }

  function slug(text) {
    return (text || 'checklist')
      .toLowerCase()
      .normalize('NFD')
      .replace(/[\u0300-\u036f]/g, '')
      .replace(/[^a-z0-9]+/g, '-')
      .replace(/^-|-$/g, '') || 'checklist';
  }

  function exportOne(checklist, format) {
    if (format === 'csv') {
      downloadText(`${slug(checklist.name)}.csv`, toCSV(checklist), 'text/csv');
    } else {
      downloadText(`${slug(checklist.name)}.json`, toJSON([checklist]));
    }
    showToast($t('checklist.exported'), 'info');
  }

  function exportAll() {
    downloadText('dcsmanager-checklists.json', toJSON($checklists));
    showToast($t('checklist.exported'), 'info');
  }

  async function onImportFile(event) {
    const file = event.currentTarget.files?.[0];
    if (!file) return;
    try {
      const text = await file.text();
      const drafts = parseImport(text, file.name);
      for (const entry of drafts) await saveChecklist(entry);
      showToast($t(drafts.length === 1 ? 'checklist.importedOne' : 'checklist.imported', { count: drafts.length }), 'success');
    } catch (error) {
      showToast(error.message, 'error');
    } finally {
      event.currentTarget.value = '';
    }
  }

  // ---- Execution ----------------------------------------------------------

  function run(checklist) {
    running = checklist;
    progress = loadProgress(checklist.uid);
    activeItem = Math.min(Math.max(progress.current ?? 0, 0), Math.max(0, checklist.items.length - 1));
  }

  function stopRun() {
    running = null;
    progress = { checked: [], current: 0 };
    activeItem = 0;
  }

  function toggle(index) {
    const item = running.items[index];
    if (!item) return;
    const checked = progress.checked.includes(item.uid);
    progress = {
      checked: checked
        ? progress.checked.filter((uid) => uid !== item.uid)
        : [...progress.checked, item.uid],
      current: index,
    };
    activeItem = index;
    saveProgress(running.uid, progress);
  }

  function go(delta) {
    if (!running) return;
    activeItem = Math.min(Math.max(activeItem + delta, 0), running.items.length - 1);
    progress = { ...progress, current: activeItem };
    saveProgress(running.uid, progress);
  }

  function resetRun() {
    if (!running) return;
    progress = { checked: [], current: 0 };
    activeItem = 0;
    clearProgress(running.uid);
  }

  function onKey(event) {
    if (!running) return;
    const target = event.target;
    if (target?.closest?.('input, textarea, select, button, a')) return;
    if (event.key === 'ArrowRight' || event.key === 'ArrowDown') {
      event.preventDefault();
      go(1);
    } else if (event.key === 'ArrowLeft' || event.key === 'ArrowUp') {
      event.preventDefault();
      go(-1);
    } else if (event.key === 'Enter') {
      event.preventDefault();
      toggle(activeItem);
    } else if (event.key.toLowerCase() === 'r') {
      event.preventDefault();
      resetRun();
    }
  }
</script>

<svelte:window on:keydown={onKey} />

<section class="checklists">
  {#if running}
    <div class="run">
      <header class="run-head">
        <div>
          <button class="btn ghost" on:click={stopRun}>← {$t('checklist.back')}</button>
          <h2>{running.name}</h2>
          <p class="meta">
            {$t(`phase.${running.phase}`)}
            {#if running.aircraft}· {running.aircraft}{/if}
          </p>
        </div>
        <div class="run-score">
          <strong>{$t('checklist.runProgress', { done: doneCount, total: totalItems })}</strong>
          {#if allDone}<Badge tone="green">{$t('checklist.done')}</Badge>{/if}
        </div>
      </header>

      <div class="bar"><span style="width:{(totalItems ? (doneCount / totalItems) * 100 : 0).toFixed(1)}%"></span></div>

      <ol class="run-list">
        {#each running.items as item, index (item.uid || index)}
          {@const checked = progress.checked.includes(item.uid)}
          <li class:checked class:current={index === activeItem}>
            <label>
              <input type="checkbox" checked={checked} on:change={() => toggle(index)} />
              <span class="check"></span>
              <span class="item-text">
                <strong>{item.text}</strong>
                {#if item.response}<span class="response">→ {item.response}</span>{/if}
                {#if item.note}<span class="note">{item.note}</span>{/if}
              </span>
              {#if item.critical}<Badge tone="red">{$t('checklist.critical')}</Badge>{/if}
            </label>
          </li>
        {/each}
      </ol>

      <footer class="run-actions">
        <button class="btn" on:click={resetRun}>{$t('checklist.reset')}</button>
        <span class="spacer"></span>
        <button class="btn" on:click={() => go(-1)} disabled={activeItem === 0}>{$t('checklist.previous')}</button>
        <button class="btn primary" on:click={() => go(1)} disabled={activeItem >= totalItems - 1}>{$t('checklist.next')}</button>
      </footer>
      <p class="shortcuts">{$t('checklist.shortcuts')}</p>
    </div>
  {:else}
    <div class="toolbar">
      <input class="search" type="search" bind:value={query} placeholder={$t('checklist.search')} aria-label={$t('checklist.search')} />
      <select bind:value={phaseFilter} aria-label={$t('checklist.phase')}>
        <option value="">{$t('checklist.allPhases')}</option>
        {#each PHASES as phase (phase)}
          <option value={phase}>{$t(`phase.${phase}`)}</option>
        {/each}
      </select>
      <select bind:value={aircraftFilter} aria-label={$t('checklist.aircraft')}>
        <option value="">{$t('checklist.allAircraft')}</option>
        {#each aircrafts as aircraft (aircraft)}
          <option value={aircraft}>{aircraft}</option>
        {/each}
      </select>
      <span class="spacer"></span>
      <button class="btn primary" on:click={startNew}>+ {$t('checklist.new')}</button>
      <button class="btn" on:click={() => importInput?.click()}>{$t('checklist.import')}</button>
      <button class="btn ghost" on:click={exportAll} disabled={$checklists.length === 0}>{$t('checklist.exportAll')}</button>
      <input class="file" type="file" accept=".json,.csv,.txt,application/json,text/csv,text/plain" bind:this={importInput} on:change={onImportFile} />
    </div>

    {#if !$checklistsEnabled}
      <EmptyState title={$t('checklist.error', { detail: '' })} message={$t('overview.storageHint')} />
    {:else if $checklistsLoading}
      <Card><Skeleton lines={6} /></Card>
    {:else if $checklistsError}
      <EmptyState title={$t('checklist.error', { detail: '' })} message={$checklistsError} />
    {:else}
      {#if visible.length === 0}
        <Card>
          <EmptyState title={$t('checklist.none')} message={$t('checklist.noneHint')} icon="M5 4h14v16H5zM8 9h8M8 13h8" />
        </Card>
      {:else}
        <div class="grid">
          {#each visible as checklist (checklist.uid)}
            <article class="item-card">
              <header>
                <div class="title">
                  <h3>{checklist.name}</h3>
                  <p>
                    {$t(`phase.${checklist.phase}`)}
                    {#if checklist.aircraft}· {checklist.aircraft}{/if}
                  </p>
                </div>
                <Badge tone={checklist.source === 'builtin' ? 'blue' : 'neutral'}>
                  {checklist.source === 'import' ? $t('checklist.sourceImport') : checklist.source === 'builtin' ? $t('checklist.sourceBuiltin') : $t('checklist.sourceUser')}
                </Badge>
                {#if checklist.visibility === 'squadron'}<Badge tone="green">{$t('checklist.visibilitySquadron')}</Badge>{/if}
              </header>
              {#if checklist.description}<p class="description">{checklist.description}</p>{/if}
              <p class="count">{$t(checklist.items.length === 1 ? 'checklist.itemCountOne' : 'checklist.itemCount', { count: checklist.items.length })}</p>
              <div class="card-actions primary">
                <button class="btn primary" on:click={() => run(checklist)}>{$t('checklist.run')}</button>
                {#if canEditChecklist(checklist)}
                  <button class="btn" on:click={() => edit(checklist)}>{$t('checklist.edit')}</button>
                {/if}
                <button class="btn ghost" on:click={() => duplicate(checklist)}>{$t('checklist.duplicate')}</button>
              </div>
              <div class="card-actions secondary">
                <button class="btn ghost" on:click={() => exportOne(checklist, 'json')}>JSON</button>
                <button class="btn ghost" on:click={() => exportOne(checklist, 'csv')}>CSV</button>
                {#if canEditChecklist(checklist)}
                  <button class="btn danger" on:click={() => remove(checklist)}>{$t('checklist.delete')}</button>
                {:else}
                  <Badge tone="amber">{$t('checklist.readOnlyShort')}</Badge>
                {/if}
              </div>
            </article>
          {/each}
        </div>
      {/if}

      <details class="templates" open={$checklists.length === 0}>
        <summary>
          {$t('checklist.templates')} <span class="badge-count">{templateCount()}</span>
          <span class="hint">{$t('checklist.templatesHint')}</span>
        </summary>
        <div class="template-grid">
          {#each templates as template (template.id)}
            <div class="template">
              <div>
                <strong>{template.name}</strong>
                <p>{$t(`phase.${template.phase}`)} · {$t(template.items.length === 1 ? 'checklist.itemCountOne' : 'checklist.itemCount', { count: template.items.length })}</p>
              </div>
              <button class="btn" on:click={() => useTemplate(template)}>{$t('checklist.useTemplate')}</button>
            </div>
          {/each}
        </div>
      </details>
    {/if}
  {/if}
</section>

<Modal
  open={editorOpen}
  title={draft?.uid ? $t('checklist.edit') : $t('checklist.new')}
  subtitle={$t('checklist.editorHint')}
  wide
  on:close={closeEditor}
>
  {#if draft}
    <div class="editor">
      <div class="fields">
        <label class="field">
          <span>{$t('checklist.name')}</span>
          <input bind:value={draft.name} maxlength="120" />
        </label>
        <label class="field">
          <span>{$t('checklist.phase')}</span>
          <select bind:value={draft.phase}>
            {#each PHASES as phase (phase)}
              <option value={phase}>{$t(`phase.${phase}`)}</option>
            {/each}
          </select>
        </label>
        <label class="field">
          <span>{$t('checklist.aircraft')}</span>
          <input bind:value={draft.aircraft} maxlength="64" placeholder={$t('checklist.aircraftPlaceholder')} />
        </label>
        <label class="field">
          <span>{$t('checklist.visibility')}</span>
          <select bind:value={draft.visibility}>
            <option value="private">{$t('checklist.visibilityPrivate')}</option>
            <option value="squadron">{$t('checklist.visibilitySquadron')}</option>
          </select>
        </label>
      </div>
      <label class="field">
        <span>{$t('checklist.description')}</span>
        <input bind:value={draft.description} maxlength="600" />
      </label>

      <div class="items">
        {#each draft.items as item, index (index)}
          <div class="item-row">
            <span class="index">{index + 1}</span>
            <div class="item-fields">
              <input bind:value={item.text} placeholder={$t('checklist.itemText')} maxlength="300" />
              <input bind:value={item.response} placeholder={$t('checklist.itemResponse')} maxlength="160" />
              <input bind:value={item.note} placeholder={$t('checklist.itemNote')} maxlength="300" />
            </div>
            <label class="critical" title={$t('checklist.critical')}>
              <input type="checkbox" bind:checked={item.critical} />
              !
            </label>
            <div class="item-buttons">
              <button class="btn ghost" on:click={() => moveItem(index, -1)} disabled={index === 0} title={$t('checklist.moveUp')}>↑</button>
              <button class="btn ghost" on:click={() => moveItem(index, 1)} disabled={index === draft.items.length - 1} title={$t('checklist.moveDown')}>↓</button>
              <button class="btn danger" on:click={() => removeItem(index)} title={$t('checklist.remove')}>×</button>
            </div>
          </div>
        {/each}
      </div>
      <button class="btn" on:click={addItem}>+ {$t('checklist.addItem')}</button>

      {#if editError}<p class="error">{editError}</p>{/if}
    </div>
  {/if}

  <div slot="footer" class="modal-actions">
    {#if draft?.uid}
      <button class="btn ghost" on:click={() => exportOne(draft, 'json')}>JSON</button>
      <button class="btn ghost" on:click={() => exportOne(draft, 'csv')}>CSV</button>
    {/if}
    <span class="spacer"></span>
    <button class="btn" on:click={closeEditor} disabled={saving}>{$t('checklist.cancel')}</button>
    <button class="btn primary" on:click={save} disabled={saving}>{$t('checklist.save')}</button>
  </div>
</Modal>

<style>
  .checklists {
    display: flex;
    flex-direction: column;
    gap: 0.7rem;
    width: 100%;
    max-width: var(--content-max);
    margin: 0 auto;
    padding: 1rem;
  }

  .toolbar {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 0.45rem;
  }

  .search {
    flex: 1;
    min-width: 170px;
    max-width: 300px;
    padding: 0.4rem 0.55rem;
    color: var(--text);
    background: var(--bg-elevated);
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-sm);
    font-size: 0.8rem;
  }

  .search:focus {
    outline: none;
    border-color: var(--blue);
  }

  select {
    padding: 0.38rem 0.45rem;
    color: var(--text);
    background: var(--bg-elevated);
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-sm);
    font-size: 0.78rem;
  }

  .spacer {
    flex: 1;
  }

  .file {
    display: none;
  }

  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(290px, 1fr));
    gap: 0.6rem;
  }

  .item-card {
    display: flex;
    flex-direction: column;
    gap: 0.45rem;
    padding: 0.8rem;
    background: var(--panel);
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    box-shadow: var(--shadow-soft);
  }

  .item-card header {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 0.5rem;
  }

  .title {
    min-width: 0;
  }

  h3 {
    margin: 0;
    font-size: 0.9rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .item-card header p,
  .template p {
    margin: 0.15rem 0 0;
    color: var(--muted);
    font-size: 0.72rem;
  }

  .description {
    margin: 0;
    color: var(--muted);
    font-size: 0.76rem;
    line-height: 1.4;
  }

  .count {
    margin: 0;
    color: var(--faint);
    font-size: 0.72rem;
  }

  .card-actions {
    display: flex;
    flex-wrap: wrap;
    gap: 0.3rem;
    margin-top: auto;
  }

  .card-actions.primary {
    padding-top: 0.25rem;
  }

  .card-actions.secondary {
    padding-top: 0.35rem;
    border-top: 1px solid var(--border);
  }

  .templates {
    margin-top: 0.4rem;
    background: var(--panel);
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
  }

  summary {
    display: flex;
    align-items: center;
    gap: 0.45rem;
    padding: 0.7rem 0.9rem;
    font-size: 0.84rem;
    font-weight: 600;
    cursor: pointer;
    list-style: none;
  }

  summary::-webkit-details-marker {
    display: none;
  }

  .badge-count {
    padding: 0.05rem 0.4rem;
    background: var(--blue-soft);
    color: var(--blue);
    border-radius: 999px;
    font-size: 0.7rem;
  }

  .hint {
    color: var(--muted);
    font-size: 0.73rem;
    font-weight: 400;
  }

  .template-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
    gap: 0.45rem;
    padding: 0 0.9rem 0.9rem;
  }

  .template {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.5rem;
    padding: 0.5rem 0.6rem;
    background: var(--bg-elevated);
    border: 1px solid var(--border);
    border-radius: var(--radius);
  }

  .template strong {
    font-size: 0.8rem;
  }

  /* Execution */
  .run {
    display: flex;
    flex-direction: column;
    gap: 0.7rem;
    max-width: 860px;
  }

  .run-head {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 0.8rem;
    flex-wrap: wrap;
  }

  .run-head h2 {
    margin: 0.25rem 0 0;
    font-size: 1.15rem;
  }

  .meta {
    margin: 0.15rem 0 0;
    color: var(--muted);
    font-size: 0.76rem;
  }

  .run-score {
    display: flex;
    flex-direction: column;
    align-items: flex-end;
    gap: 0.25rem;
  }

  .run-score strong {
    font-size: 1rem;
    font-variant-numeric: tabular-nums;
  }

  .bar {
    height: 7px;
    background: var(--bg-elevated);
    border: 1px solid var(--border);
    border-radius: 999px;
    overflow: hidden;
  }

  .bar span {
    display: block;
    height: 100%;
    background: linear-gradient(90deg, var(--blue), var(--green));
    transition: width 160ms ease;
  }

  .run-list {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
  }

  .run-list li {
    background: var(--panel);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    transition: border-color 120ms ease;
  }

  .run-list li.current {
    border-color: var(--blue);
    box-shadow: 0 0 0 1px var(--blue-soft);
  }

  .run-list li.checked {
    opacity: 0.62;
  }

  .run-list li.checked .item-text strong {
    text-decoration: line-through;
  }

  .run-list label {
    display: flex;
    align-items: center;
    gap: 0.7rem;
    padding: 0.75rem 0.8rem;
    cursor: pointer;
    min-height: 52px;
  }

  .run-list input {
    position: absolute;
    opacity: 0;
    pointer-events: none;
  }

  .check {
    width: 24px;
    height: 24px;
    flex: none;
    border: 2px solid var(--border-strong);
    border-radius: 7px;
    position: relative;
  }

  .run-list input:checked + .check {
    background: var(--green);
    border-color: var(--green);
  }

  .run-list input:checked + .check::after {
    content: '✓';
    position: absolute;
    inset: 0;
    display: grid;
    place-items: center;
    color: #04120a;
    font-size: 0.95rem;
    font-weight: 800;
  }

  .run-list input:focus-visible + .check {
    outline: 2px solid var(--blue);
    outline-offset: 2px;
  }

  .item-text {
    display: flex;
    flex-direction: column;
    gap: 0.12rem;
    min-width: 0;
    flex: 1;
  }

  .item-text strong {
    font-size: 0.9rem;
    font-weight: 600;
  }

  .response {
    color: var(--blue);
    font-size: 0.8rem;
  }

  .note {
    color: var(--muted);
    font-size: 0.74rem;
  }

  .run-actions {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    flex-wrap: wrap;
  }

  .shortcuts {
    margin: 0;
    color: var(--faint);
    font-size: 0.72rem;
  }

  /* Editor */
  .editor {
    display: flex;
    flex-direction: column;
    gap: 0.7rem;
  }

  .fields {
    display: grid;
    grid-template-columns: 2fr 1fr 1fr 1fr;
    gap: 0.5rem;
  }

  .items {
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
    max-height: 46vh;
    overflow-y: auto;
    padding-right: 0.2rem;
  }

  .item-row {
    display: flex;
    align-items: flex-start;
    gap: 0.35rem;
    padding: 0.4rem;
    background: var(--bg-elevated);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
  }

  .index {
    width: 20px;
    flex: none;
    padding-top: 0.45rem;
    text-align: center;
    color: var(--faint);
    font-size: 0.72rem;
  }

  .item-fields {
    display: grid;
    grid-template-columns: 2fr 1fr 1.2fr;
    gap: 0.3rem;
    flex: 1;
    min-width: 0;
  }

  .item-fields input {
    width: 100%;
    padding: 0.35rem 0.45rem;
    color: var(--text);
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    font-size: 0.78rem;
  }

  .item-fields input:focus {
    outline: none;
    border-color: var(--blue);
  }

  .critical {
    display: flex;
    align-items: center;
    gap: 0.2rem;
    padding-top: 0.35rem;
    color: var(--red);
    font-weight: 700;
    font-size: 0.8rem;
  }

  .critical input {
    accent-color: var(--red);
  }

  .item-buttons {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .item-buttons .btn {
    padding: 0.1rem 0.32rem;
    line-height: 1.2;
  }

  .error {
    margin: 0;
    color: var(--red);
    font-size: 0.78rem;
  }

  .modal-actions {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    width: 100%;
  }

  @media (max-width: 820px) {
    .fields {
      grid-template-columns: 1fr 1fr;
    }

    .item-fields {
      grid-template-columns: 1fr;
    }
  }

  @media (max-width: 620px) {
    .checklists {
      padding: 0.7rem;
    }

    .fields {
      grid-template-columns: 1fr;
    }
  }
</style>
