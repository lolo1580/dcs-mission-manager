<script>
  import { onMount } from 'svelte';
  import Card from './ui/Card.svelte';
  import Badge from './ui/Badge.svelte';
  import EmptyState from './ui/EmptyState.svelte';
  import Skeleton from './ui/Skeleton.svelte';
  import SchoolEditor from './SchoolEditor.svelte';
  import { schoolAPI } from './school.js';
  import { renderMarkdown } from './markdown.js';
  import { t, lang } from './i18n.js';
  import { sessionUser, isAdmin } from './session.js';

  let taxonomy = [];
  let items = [];
  let progress = {};
  let loading = true;
  let error = '';
  let selected = null;
  let detailLoading = false;
  let editorOpen = false;
  let editorItem = null;
  let query = '';
  let domain = '';
  let era = '';

  $: domains = taxonomy.filter((entry) => entry.kind === 'domain');
  $: eras = taxonomy.filter((entry) => entry.kind === 'era');
  $: visible = items.filter((item) => {
    if (domain && item.domain !== domain) return false;
    if (era && item.era !== era) return false;
    if (query) {
      const haystack = `${item.title} ${item.designation} ${item.summary}`.toLowerCase();
      if (!haystack.includes(query.toLowerCase())) return false;
    }
    return true;
  });
  $: canEdit = selected && ($isAdmin || selected.authorId === $sessionUser?.id);
  $: selectedProgress = selected ? progress[selected.uid] : null;


  function label(entry) {
    return $lang === 'fr' ? entry.labelFr : entry.labelEn;
  }

  async function load() {
    loading = true;
    error = '';
    try {
      const [tax, list, prog] = await Promise.all([
        schoolAPI.taxonomy(),
        schoolAPI.items({ kind: 'lesson', limit: 200 }),
        schoolAPI.progress(),
      ]);
      taxonomy = tax.taxonomy ?? [];
      items = list.items ?? [];
      progress = Object.fromEntries((prog.progress ?? []).map((entry) => [entry.itemUid, entry]));
      if (selected) selected = items.find((item) => item.uid === selected.uid) ?? null;
    } catch (err) {
      error = err.message;
    } finally {
      loading = false;
    }
  }

  async function open(item) {
    detailLoading = true;
    try {
      const result = await schoolAPI.item(item.uid);
      selected = result.item;
    } catch (err) {
      error = err.message;
    } finally {
      detailLoading = false;
    }
  }

  async function mark(status) {
    if (!selected) return;
    try {
      await schoolAPI.setProgress(selected.uid, status, status === 'completed' ? 100 : 0);
      progress = {
        ...progress,
        [selected.uid]: { itemUid: selected.uid, status, percent: status === 'completed' ? 100 : 0 },
      };
    } catch (err) {
      error = err.message;
    }
  }

  function create() {
    editorItem = null;
    editorOpen = true;
  }

  function edit() {
    editorItem = selected;
    editorOpen = true;
  }

  async function onSaved(saved) {
    await load();
    if (saved?.uid) await open(saved);
  }

  onMount(load);
</script>

<section class="school">
  <header class="head">
    <div>
      <h1>{$t('school.title')}</h1>
      <p>{$t('school.subtitle')}</p>
    </div>
    <button class="btn primary" on:click={create}>{$t('school.create')}</button>
  </header>

  {#if error}<p class="error">{$t('school.error', { detail: error })}</p>{/if}

  <div class="filters">
    <input bind:value={query} placeholder={$t('school.search')} />
    <select bind:value={domain}>
      <option value="">{$t('school.allDomains')}</option>
      {#each domains as entry (entry.slug)}<option value={entry.slug}>{label(entry)}</option>{/each}
    </select>
    <select bind:value={era}>
      <option value="">{$t('school.allEras')}</option>
      {#each eras as entry (entry.slug)}<option value={entry.slug}>{label(entry)}</option>{/each}
    </select>
  </div>

  {#if loading}
    <div class="layout">
      <Card><Skeleton lines={8} /></Card>
      <Card><Skeleton lines={5} /></Card>
    </div>
  {:else}
    <div class="layout">
      <Card title={$t('school.lessons')} subtitle={$t('school.count', { count: visible.length })}>
        {#if visible.length === 0}
          <EmptyState title={$t('school.empty')} message={$t('school.emptyHint')} />
        {:else}
          <ul class="lesson-list">
            {#each visible as item (item.uid)}
              <li>
                <button class:active={selected?.uid === item.uid} on:click={() => open(item)}>
                  <span class="line">
                    <strong>{item.title}</strong>
                    {#if progress[item.uid]?.status === 'completed'}<Badge tone="green">{$t('school.done')}</Badge>{/if}
                    {#if item.status !== 'published'}<Badge tone="amber">{$t('school.pending')}</Badge>{/if}
                  </span>
                  <span class="meta">
                    {item.domain ? label(domains.find((d) => d.slug === item.domain) ?? { labelFr: item.domain, labelEn: item.domain }) : '—'}
                    {#if item.era}· {label(eras.find((e) => e.slug === item.era) ?? { labelFr: item.era, labelEn: item.era })}{/if}
                  </span>
                  {#if item.summary}<span class="summary">{item.summary}</span>{/if}
                </button>
              </li>
            {/each}
          </ul>
        {/if}
      </Card>

      <Card title={selected?.title ?? $t('school.select')}>
        {#if detailLoading}
          <Skeleton lines={8} />
        {:else if !selected}
          <EmptyState title={$t('school.select')} message={$t('school.selectHint')} />
        {:else}
          <div class="detail">
            <div class="line">
              <Badge tone="blue">{$t('school.lesson')}</Badge>
              {#if selected.status !== 'published'}<Badge tone="amber">{$t('school.pending')}</Badge>{/if}
              {#if selected.author}<span class="author">{$t('school.by', { name: selected.author })}</span>{/if}
            </div>
            {#if selected.summary}<p class="lead">{selected.summary}</p>{/if}
            <div class="markdown">{@html renderMarkdown(selected.bodyMd)}</div>
            {#if selected.specs?.length}
              <table class="specs">
                <tbody>
                  {#each selected.specs as spec (spec.label)}
                    <tr><th>{spec.label}</th><td>{spec.value} {spec.unit ?? ''}</td></tr>
                  {/each}
                </tbody>
              </table>
            {/if}
            {#if selected.sources?.length}
              <div class="sources">
                <span>{$t('school.sources')}</span>
                {#each selected.sources as source (source.url)}
                  <a href={source.url} target="_blank" rel="noopener noreferrer">{source.label}</a>
                {/each}
              </div>
            {/if}
            <div class="actions">
              {#if selectedProgress?.status !== 'completed'}
                <button class="btn primary" on:click={() => mark('completed')}>{$t('school.markDone')}</button>
              {:else}
                <button class="btn ghost" on:click={() => mark('in_progress')}>{$t('school.markProgress')}</button>
              {/if}
              {#if canEdit}<button class="btn ghost" on:click={edit}>{$t('school.edit')}</button>{/if}
            </div>
          </div>
        {/if}
      </Card>
    </div>
  {/if}
</section>

<SchoolEditor
  bind:open={editorOpen}
  kind="lesson"
  item={editorItem}
  {taxonomy}
  on:close={() => (editorOpen = false)}
  on:saved={(event) => onSaved(event.detail)}
/>

<style>
  .school {
    display: flex;
    flex-direction: column;
    gap: 0.8rem;
  }
  .head {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 1rem;
  }
  .head h1 {
    margin: 0;
    font-size: 1.2rem;
  }
  .head p {
    margin: 0.2rem 0 0;
    color: var(--muted);
    font-size: 0.82rem;
  }
  .filters {
    display: grid;
    grid-template-columns: 1fr 200px 200px;
    gap: 0.5rem;
  }
  .filters input,
  .filters select {
    padding: 0.4rem 0.6rem;
    border-radius: var(--radius);
    border: 1px solid var(--border);
    background: var(--panel);
    color: var(--text);
    font: inherit;
  }
  .layout {
    display: grid;
    grid-template-columns: minmax(280px, 0.9fr) minmax(0, 1.6fr);
    gap: 0.8rem;
    align-items: start;
  }
  .lesson-list {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
  }
  .lesson-list button {
    width: 100%;
    text-align: left;
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
    padding: 0.5rem 0.6rem;
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--bg-elevated);
    color: var(--text);
    cursor: pointer;
  }
  .lesson-list button:hover,
  .lesson-list button.active {
    border-color: color-mix(in srgb, var(--blue) 45%, var(--border));
  }
  .line {
    display: flex;
    align-items: center;
    gap: 0.35rem;
    flex-wrap: wrap;
  }
  .meta,
  .author {
    color: var(--muted);
    font-size: 0.72rem;
  }
  .summary {
    color: var(--muted);
    font-size: 0.78rem;
  }
  .detail {
    display: flex;
    flex-direction: column;
    gap: 0.7rem;
  }
  .lead {
    margin: 0;
    color: var(--muted);
  }
  .markdown {
    font-size: 0.9rem;
    line-height: 1.6;
  }
  .markdown :global(h1),
  .markdown :global(h2),
  .markdown :global(h3) {
    margin: 1rem 0 0.4rem;
  }
  .markdown :global(p) {
    margin: 0.45rem 0;
  }
  .markdown :global(ul),
  .markdown :global(ol) {
    padding-left: 1.3rem;
  }
  .markdown :global(a) {
    color: var(--blue);
  }
  .specs {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.82rem;
  }
  .specs th,
  .specs td {
    text-align: left;
    padding: 0.3rem 0.5rem;
    border-bottom: 1px solid var(--border);
  }
  .specs th {
    color: var(--muted);
    font-weight: 500;
    width: 40%;
  }
  .sources {
    display: flex;
    flex-wrap: wrap;
    gap: 0.5rem;
    font-size: 0.8rem;
  }
  .sources span {
    color: var(--muted);
  }
  .sources a {
    color: var(--blue);
  }
  .actions {
    display: flex;
    gap: 0.5rem;
    flex-wrap: wrap;
  }
  .error {
    margin: 0;
    color: var(--red);
  }
  @media (max-width: 900px) {
    .filters,
    .layout {
      grid-template-columns: 1fr;
    }
  }
</style>
