<script>
  import { createEventDispatcher } from 'svelte';
  import Modal from './ui/Modal.svelte';
  import { schoolAPI } from './school.js';
  import { t, lang } from './i18n.js';

  export let open = false;
  export let kind = 'card';
  export let item = null;
  export let taxonomy = [];

  const dispatch = createEventDispatcher();

  let title = '';
  let designation = '';
  let summary = '';
  let bodyMd = '';
  let domain = '';
  let era = '';
  let country = '';
  let manufacturer = '';
  let serviceStart = '';
  let serviceEnd = '';
  let tags = '';
  let imageUrl = '';
  let specsText = '';
  let sourcesText = '';
  let linksText = '';
  let questions = [];
  let saving = false;
  let error = '';
  let lastKey = '';

  $: domains = taxonomy.filter((entry) => entry.kind === 'domain');
  $: eras = taxonomy.filter((entry) => entry.kind === 'era');
  $: if (!open) {
    lastKey = '';
  } else {
    const key = `${item?.uid ?? 'new'}:${kind}`;
    if (key !== lastKey) {
      lastKey = key;
      hydrate();
    }
  }

  function blankQuestion() {
    return { prompt: '', choices: ['', '', '', ''], correctIndex: 0, explanation: '' };
  }

  function hydrate() {
    error = '';
    saving = false;
    title = item?.title ?? '';
    designation = item?.designation ?? '';
    summary = item?.summary ?? '';
    bodyMd = item?.bodyMd ?? '';
    domain = item?.domain ?? '';
    era = item?.era ?? '';
    country = item?.country ?? '';
    manufacturer = item?.manufacturer ?? '';
    serviceStart = item?.serviceStart || '';
    serviceEnd = item?.serviceEnd || '';
    tags = (item?.tags ?? []).join(', ');
    imageUrl = item?.imageUrl ?? '';
    specsText = (item?.specs ?? []).map((s) => [s.label, s.value, s.unit].filter(Boolean).join(' | ')).join('\n');
    sourcesText = (item?.sources ?? []).map((s) => `${s.label} | ${s.url}`).join('\n');
    linksText = (item?.links ?? []).join(', ');
    questions = (item?.questions ?? []).map((q) => ({
      prompt: q.prompt,
      choices: [...q.choices, '', '', ''].slice(0, Math.max(4, q.choices.length)),
      correctIndex: q.correctIndex ?? 0,
      explanation: q.explanation ?? '',
    }));
    if (kind === 'quiz' && questions.length === 0) questions = [blankQuestion()];
  }

  function parseSpecs(text) {
    return text
      .split('\n')
      .map((line) => line.split('|').map((part) => part.trim()))
      .filter((parts) => parts[0] && parts[1])
      .map(([label, value, unit = '']) => ({ label, value, unit }));
  }

  function parseSources(text) {
    return text
      .split('\n')
      .map((line) => line.split('|').map((part) => part.trim()))
      .filter((parts) => parts[0] && parts[1])
      .map(([label, url]) => ({ label, url }));
  }

  function addQuestion() {
    questions = [...questions, blankQuestion()];
  }

  function removeQuestion(index) {
    questions = questions.filter((_, i) => i !== index);
  }

  function choiceTexts(q) {
    return q.choices.map((choice) => choice.trim()).filter(Boolean);
  }

  function buildPayload() {
    const payload = {
      kind,
      title: title.trim(),
      designation: designation.trim(),
      summary: summary.trim(),
      bodyMd,
      domain,
      era,
      country: country.trim(),
      manufacturer: manufacturer.trim(),
      serviceStart: Number(serviceStart) || 0,
      serviceEnd: Number(serviceEnd) || 0,
      tags: tags.split(',').map((tag) => tag.trim()).filter(Boolean),
      imageUrl: imageUrl.trim(),
      specs: parseSpecs(specsText),
      sources: parseSources(sourcesText),
      links: linksText.split(',').map((uid) => uid.trim()).filter(Boolean),
    };
    if (kind === 'quiz') {
      payload.questions = questions.map((q) => {
        const choices = choiceTexts(q);
        const correct = q.choices[q.correctIndex]?.trim() ?? '';
        return {
          prompt: q.prompt.trim(),
          choices,
          correctIndex: choices.indexOf(correct),
          explanation: q.explanation.trim(),
        };
      });
    }
    return payload;
  }

  async function save() {
    error = '';
    saving = true;
    try {
      const payload = buildPayload();
      if (payload.title === '') throw new Error($t('school.editor.titleRequired'));
      if (kind === 'quiz') {
        for (const q of payload.questions) {
          if (!q.prompt || q.choices.length < 2 || q.correctIndex < 0) {
            throw new Error($t('school.editor.quizInvalid'));
          }
        }
      }
      const result = item?.uid
        ? await schoolAPI.update(item.uid, payload)
        : await schoolAPI.create(payload);
      dispatch('saved', result.item);
      dispatch('close');
    } catch (err) {
      error = err.message;
    } finally {
      saving = false;
    }
  }
</script>

<Modal {open} title={item ? $t('school.editor.edit') : $t('school.editor.create')} subtitle={$t('school.editor.hint')} wide on:close={() => dispatch('close')}>
  <div class="editor">
    {#if item?.revisionPending}
      <p class="notice">{$t('school.editor.revisionPending')}</p>
    {/if}
    <div class="row two">
      <label>
        <span>{$t('school.editor.title')}</span>
        <input bind:value={title} maxlength="200" required />
      </label>
      <label>
        <span>{$t('school.editor.designation')}</span>
        <input bind:value={designation} maxlength="120" />
      </label>
    </div>
    <label>
      <span>{$t('school.editor.summary')}</span>
      <input bind:value={summary} maxlength="600" />
    </label>
    <div class="row three">
      <label>
        <span>{$t('school.editor.domain')}</span>
        <select bind:value={domain}>
          <option value="">—</option>
          {#each domains as entry (entry.slug)}
            <option value={entry.slug}>{$lang === 'fr' ? entry.labelFr : entry.labelEn}</option>
          {/each}
        </select>
      </label>
      <label>
        <span>{$t('school.editor.era')}</span>
        <select bind:value={era}>
          <option value="">—</option>
          {#each eras as entry (entry.slug)}
            <option value={entry.slug}>{$lang === 'fr' ? entry.labelFr : entry.labelEn}</option>
          {/each}
        </select>
      </label>
      <label>
        <span>{$t('school.editor.country')}</span>
        <input bind:value={country} maxlength="80" />
      </label>
    </div>
    <div class="row three">
      <label>
        <span>{$t('school.editor.manufacturer')}</span>
        <input bind:value={manufacturer} maxlength="120" />
      </label>
      <label>
        <span>{$t('school.editor.serviceStart')}</span>
        <input bind:value={serviceStart} type="number" min="1000" max="2200" />
      </label>
      <label>
        <span>{$t('school.editor.serviceEnd')}</span>
        <input bind:value={serviceEnd} type="number" min="1000" max="2200" />
      </label>
    </div>
    <div class="row two">
      <label>
        <span>{$t('school.editor.tags')}</span>
        <input bind:value={tags} placeholder="F-16, USAF, chasseur" />
      </label>
      <label>
        <span>{$t('school.editor.image')}</span>
        <input bind:value={imageUrl} placeholder="https://…" />
      </label>
    </div>
    <label>
      <span>{$t('school.editor.body')}</span>
      <textarea bind:value={bodyMd} rows="8" placeholder="# Titre&#10;Texte en Markdown…"></textarea>
    </label>
    <div class="row two">
      <label>
        <span>{$t('school.editor.specs')}</span>
        <textarea bind:value={specsText} rows="5" placeholder="Vitesse max | 2120 | km/h"></textarea>
      </label>
      <label>
        <span>{$t('school.editor.sources')}</span>
        <textarea bind:value={sourcesText} rows="5" placeholder="USAF | https://www.af.mil"></textarea>
      </label>
    </div>
    <label>
      <span>{$t('school.editor.links')}</span>
      <input bind:value={linksText} placeholder="sc_xxx, sc_yyy" />
    </label>

    {#if kind === 'quiz'}
      <section class="questions">
        <header>
          <h3>{$t('school.editor.questions')}</h3>
          <button type="button" class="btn ghost" on:click={addQuestion}>{$t('school.editor.addQuestion')}</button>
        </header>
        {#each questions as q, qi (qi)}
          <div class="question">
            <div class="question-head">
              <strong># {qi + 1}</strong>
              {#if questions.length > 1}
                <button type="button" class="btn danger small" on:click={() => removeQuestion(qi)}>×</button>
              {/if}
            </div>
            <input bind:value={q.prompt} placeholder={$t('school.editor.question')} />
            <div class="choices">
              {#each q.choices as _, ci (ci)}
                <label class="choice">
                  <input type="radio" name="correct-{qi}" checked={q.correctIndex === ci} on:change={() => { q.correctIndex = ci; questions = questions; }} />
                  <input bind:value={q.choices[ci]} placeholder={$t('school.editor.choice', { n: ci + 1 })} />
                </label>
              {/each}
            </div>
            <input bind:value={q.explanation} placeholder={$t('school.editor.explanation')} />
          </div>
        {/each}
      </section>
    {/if}

    {#if error}<p class="error">{error}</p>{/if}
  </div>
  <svelte:fragment slot="footer">
    <button type="button" class="btn ghost" on:click={() => dispatch('close')}>{$t('common.cancel')}</button>
    <button type="button" class="btn primary" on:click={save} disabled={saving}>
      {saving ? $t('common.saving') : $t('common.save')}
    </button>
  </svelte:fragment>
</Modal>

<style>
  .editor {
    display: flex;
    flex-direction: column;
    gap: 0.7rem;
  }
  .row {
    display: grid;
    gap: 0.7rem;
  }
  .row.two {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .row.three {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
  label {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
    font-size: 0.78rem;
    color: var(--muted);
  }
  input,
  select,
  textarea {
    width: 100%;
    padding: 0.45rem 0.6rem;
    border-radius: var(--radius);
    border: 1px solid var(--border);
    background: var(--bg-elevated);
    color: var(--text);
    font: inherit;
  }
  textarea {
    resize: vertical;
    font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
    font-size: 0.82rem;
  }
  .questions {
    border-top: 1px solid var(--border);
    padding-top: 0.7rem;
    display: flex;
    flex-direction: column;
    gap: 0.6rem;
  }
  .questions header {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }
  .questions h3 {
    margin: 0;
    font-size: 0.95rem;
  }
  .question {
    border: 1px solid var(--border);
    border-radius: var(--radius);
    padding: 0.6rem;
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
    background: var(--bg-elevated);
  }
  .question-head {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }
  .choices {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 0.35rem;
  }
  .choice {
    flex-direction: row;
    align-items: center;
    gap: 0.35rem;
    color: var(--text);
  }
  .choice input[type='radio'] {
    width: auto;
  }
  .notice {
    margin: 0;
    padding: 0.5rem 0.7rem;
    border-radius: var(--radius);
    background: color-mix(in srgb, var(--amber) 14%, transparent);
    color: var(--amber);
    font-size: 0.8rem;
  }
  .error {
    margin: 0;
    color: var(--red);
  }
  .btn.small {
    padding: 0.1rem 0.45rem;
  }
  @media (max-width: 720px) {
    .row.two,
    .row.three,
    .choices {
      grid-template-columns: 1fr;
    }
  }
</style>
