<script>
  import { onMount } from 'svelte';
  import Card from './ui/Card.svelte';
  import Badge from './ui/Badge.svelte';
  import EmptyState from './ui/EmptyState.svelte';
  import Skeleton from './ui/Skeleton.svelte';
  import { schoolAPI } from './school.js';
  import { t, lang } from './i18n.js';

  let pending = [];
  let taxonomy = [];
  let loading = true;
  let error = '';
  let saving = false;
  let form = { kind: 'domain', slug: '', labelFr: '', labelEn: '', position: 0 };

  $: domains = taxonomy.filter((entry) => entry.kind === 'domain');
  $: eras = taxonomy.filter((entry) => entry.kind === 'era');

  function label(entry) {
    return $lang === 'fr' ? entry.labelFr : entry.labelEn;
  }

  async function load() {
    loading = true;
    error = '';
    try {
      const [queue, tax] = await Promise.all([schoolAPI.pending(), schoolAPI.taxonomy()]);
      pending = queue.items ?? [];
      taxonomy = tax.taxonomy ?? [];
    } catch (err) {
      error = err.message;
    } finally {
      loading = false;
    }
  }

  async function review(item, decision) {
    let note = '';
    if (decision === 'reject') {
      const answer = prompt($t('admin.school.reason'));
      if (answer === null) return;
      note = answer;
    }
    try {
      await schoolAPI.review(item.uid, decision, note);
      await load();
    } catch (err) {
      error = err.message;
    }
  }

  async function addTaxonomy() {
    error = '';
    saving = true;
    try {
      await schoolAPI.saveTaxonomy({
        ...form,
        slug: form.slug.trim().toLowerCase(),
        labelFr: form.labelFr.trim(),
        labelEn: form.labelEn.trim(),
        position: Number(form.position) || 0,
      });
      form = { kind: 'domain', slug: '', labelFr: '', labelEn: '', position: 0 };
      await load();
    } catch (err) {
      error = err.message;
    } finally {
      saving = false;
    }
  }

  async function removeTaxonomy(entry) {
    if (!confirm($t('admin.school.confirmDelete', { name: label(entry) }))) return;
    try {
      await schoolAPI.deleteTaxonomy(entry.id);
      await load();
    } catch (err) {
      error = err.message;
    }
  }

  onMount(load);
</script>

<div class="school-admin">
  {#if error}<p class="error">{$t('admin.school.error', { detail: error })}</p>{/if}

  <Card title={$t('admin.school.pending')} subtitle={$t('admin.school.pendingHint')}>
    {#if loading}
      <Skeleton lines={4} />
    {:else if pending.length === 0}
      <EmptyState title={$t('admin.school.noPending')} message="" />
    {:else}
      <ul class="pending">
        {#each pending as item (item.uid)}
          <li>
            <div>
              <div class="line">
                <strong>{item.title}</strong>
                <Badge tone="blue">{item.kind}</Badge>
                {#if item.revisionPending}<Badge tone="amber">{$t('admin.school.revision')}</Badge>{/if}
              </div>
              <span>{item.author ? $t('school.by', { name: item.author }) : ''} {item.summary ? `· ${item.summary}` : ''}</span>
            </div>
            <div class="actions">
              <button class="btn primary" on:click={() => review(item, 'approve')}>{$t('admin.school.approve')}</button>
              <button class="btn danger" on:click={() => review(item, 'reject')}>{$t('admin.school.reject')}</button>
            </div>
          </li>
        {/each}
      </ul>
    {/if}
  </Card>

  <Card title={$t('admin.school.taxonomy')} subtitle={$t('admin.school.taxonomyHint')}>
    <form class="tax-form" on:submit|preventDefault={addTaxonomy}>
      <select bind:value={form.kind}>
        <option value="domain">{$t('admin.school.kindDomain')}</option>
        <option value="era">{$t('admin.school.kindEra')}</option>
      </select>
      <input bind:value={form.slug} placeholder={$t('admin.school.slug')} required />
      <input bind:value={form.labelFr} placeholder={$t('admin.school.labelFr')} required />
      <input bind:value={form.labelEn} placeholder={$t('admin.school.labelEn')} required />
      <input bind:value={form.position} type="number" min="0" placeholder="#" />
      <button class="btn primary" type="submit" disabled={saving}>{$t('admin.school.add')}</button>
    </form>

    {#if taxonomy.length === 0}
      <EmptyState title={$t('admin.school.noTaxonomy')} message="" />
    {:else}
      <div class="taxonomy">
        <div>
          <h3>{$t('admin.school.kindDomain')}</h3>
          <ul>
            {#each domains as entry (entry.id)}
              <li>
                <span>{label(entry)} <small>{entry.slug}</small></span>
                <button class="btn ghost small" on:click={() => removeTaxonomy(entry)}>×</button>
              </li>
            {/each}
          </ul>
        </div>
        <div>
          <h3>{$t('admin.school.kindEra')}</h3>
          <ul>
            {#each eras as entry (entry.id)}
              <li>
                <span>{label(entry)} <small>{entry.slug}</small></span>
                <button class="btn ghost small" on:click={() => removeTaxonomy(entry)}>×</button>
              </li>
            {/each}
          </ul>
        </div>
      </div>
    {/if}
  </Card>
</div>

<style>
  .school-admin {
    display: flex;
    flex-direction: column;
    gap: 0.8rem;
  }
  .pending {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
  }
  .pending li {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.7rem;
    padding: 0.45rem 0.55rem;
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--bg-elevated);
  }
  .pending li > div {
    min-width: 0;
  }
  .pending span {
    color: var(--muted);
    font-size: 0.75rem;
  }
  .line {
    display: flex;
    align-items: center;
    gap: 0.35rem;
    flex-wrap: wrap;
  }
  .actions {
    display: flex;
    gap: 0.35rem;
    flex: none;
  }
  .tax-form {
    display: grid;
    grid-template-columns: 130px 1fr 1fr 1fr 70px auto;
    gap: 0.35rem;
    margin-bottom: 0.7rem;
  }
  .tax-form input,
  .tax-form select {
    padding: 0.35rem 0.5rem;
    border-radius: var(--radius);
    border: 1px solid var(--border);
    background: var(--bg-elevated);
    color: var(--text);
    font: inherit;
  }
  .taxonomy {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 1rem;
  }
  .taxonomy h3 {
    margin: 0 0 0.3rem;
    font-size: 0.8rem;
    color: var(--muted);
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }
  .taxonomy ul {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
  }
  .taxonomy li {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.4rem;
    padding: 0.2rem 0.35rem;
    border-radius: var(--radius);
    background: var(--bg-elevated);
    font-size: 0.8rem;
  }
  .taxonomy small {
    color: var(--muted);
  }
  .btn.small {
    padding: 0.05rem 0.4rem;
  }
  .error {
    margin: 0;
    color: var(--red);
  }
  @media (max-width: 800px) {
    .tax-form,
    .taxonomy {
      grid-template-columns: 1fr;
    }
  }
</style>
