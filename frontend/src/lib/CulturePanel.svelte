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

  let tab = 'cards';
  let taxonomy = [];
  let cards = [];
  let quizzes = [];
  let progress = {};
  let board = [];
  let loading = true;
  let error = '';
  let query = '';
  let domain = '';
  let era = '';
  let selected = null;
  let selectedKind = '';
  let detailLoading = false;
  let answers = [];
  let quizResult = null;
  let quizError = '';
  let submitting = false;
  let editorOpen = false;
  let editorItem = null;
  let editorKind = 'card';

  $: domains = taxonomy.filter((entry) => entry.kind === 'domain');
  $: eras = taxonomy.filter((entry) => entry.kind === 'era');
  $: visibleCards = filterItems(cards);
  $: visibleQuizzes = filterItems(quizzes);
  $: canEdit = selected && ($isAdmin || selected.authorId === $sessionUser?.id);
  $: selectedProgress = selected ? progress[selected.uid] : null;

  function label(entry) {
    return $lang === 'fr' ? entry.labelFr : entry.labelEn;
  }

  function filterItems(list) {
    return list.filter((item) => {
      if (domain && item.domain !== domain) return false;
      if (era && item.era !== era) return false;
      if (query) {
        const haystack = `${item.title} ${item.designation} ${item.summary}`.toLowerCase();
        if (!haystack.includes(query.toLowerCase())) return false;
      }
      return true;
    });
  }

  async function load() {
    loading = true;
    error = '';
    try {
      const [tax, cardList, quizList, prog, ranking] = await Promise.all([
        schoolAPI.taxonomy(),
        schoolAPI.items({ kind: 'card', limit: 200 }),
        schoolAPI.items({ kind: 'quiz', limit: 200 }),
        schoolAPI.progress(),
        schoolAPI.leaderboard(),
      ]);
      taxonomy = tax.taxonomy ?? [];
      cards = cardList.items ?? [];
      quizzes = quizList.items ?? [];
      progress = Object.fromEntries((prog.progress ?? []).map((entry) => [entry.itemUid, entry]));
      board = ranking.leaderboard ?? [];
    } catch (err) {
      error = err.message;
    } finally {
      loading = false;
    }
  }

  async function open(item, kind) {
    detailLoading = true;
    quizResult = null;
    quizError = '';
    try {
      const result = await schoolAPI.item(item.uid);
      selected = result.item;
      selectedKind = kind;
      answers = Array(result.item.questions?.length ?? 0).fill(-1);
    } catch (err) {
      error = err.message;
    } finally {
      detailLoading = false;
    }
  }

  async function markRead() {
    if (!selected) return;
    try {
      await schoolAPI.setProgress(selected.uid, 'completed', 100);
      progress = { ...progress, [selected.uid]: { itemUid: selected.uid, status: 'completed', percent: 100 } };
    } catch (err) {
      error = err.message;
    }
  }

  async function submitQuiz() {
    if (!selected) return;
    quizError = '';
    if (answers.some((answer) => answer < 0)) {
      quizError = $t('culture.answerAll');
      return;
    }
    submitting = true;
    try {
      const result = await schoolAPI.submitQuiz(selected.uid, answers);
      quizResult = result.attempt;
      const [prog, ranking] = await Promise.all([schoolAPI.progress(), schoolAPI.leaderboard()]);
      progress = Object.fromEntries((prog.progress ?? []).map((entry) => [entry.itemUid, entry]));
      board = ranking.leaderboard ?? [];
    } catch (err) {
      quizError = err.message;
    } finally {
      submitting = false;
    }
  }

  function create() {
    editorKind = tab === 'quizzes' ? 'quiz' : 'card';
    editorItem = null;
    editorOpen = true;
  }

  function edit() {
    editorKind = selected.kind;
    editorItem = selected;
    editorOpen = true;
  }

  async function onSaved(saved) {
    await load();
    if (saved?.uid) await open(saved, saved.kind);
  }

  onMount(load);
</script>

<section class="culture">
  <header class="head">
    <div>
      <h1>{$t('culture.title')}</h1>
      <p>{$t('culture.subtitle')}</p>
    </div>
    <button class="btn primary" on:click={create}>{$t('culture.create')}</button>
  </header>

  <nav class="tabs">
    <button class:active={tab === 'cards'} on:click={() => (tab = 'cards')}>{$t('culture.cards')}</button>
    <button class:active={tab === 'quizzes'} on:click={() => (tab = 'quizzes')}>{$t('culture.quizzes')}</button>
    <button class:active={tab === 'board'} on:click={() => (tab = 'board')}>{$t('culture.leaderboard')}</button>
  </nav>

  {#if error}<p class="error">{$t('culture.error', { detail: error })}</p>{/if}

  {#if tab === 'board'}
    <Card title={$t('culture.leaderboard')} subtitle={$t('culture.leaderboardHint')}>
      {#if board.length === 0}
        <EmptyState title={$t('culture.noScores')} message="" />
      {:else}
        <table class="board">
          <thead><tr><th>#</th><th>{$t('culture.pilot')}</th><th>{$t('culture.lessons')}</th><th>{$t('culture.cardsRead')}</th><th>{$t('culture.quizPoints')}</th><th>{$t('culture.points')}</th></tr></thead>
          <tbody>
            {#each board as entry, index (entry.userId)}
              <tr class:me={entry.userId === $sessionUser?.id}>
                <td>{index + 1}</td>
                <td>{entry.displayName || entry.username}</td>
                <td>{entry.lessons}</td>
                <td>{entry.cards}</td>
                <td>{entry.quizPoints}</td>
                <td><strong>{entry.points}</strong></td>
              </tr>
            {/each}
          </tbody>
        </table>
      {/if}
    </Card>
  {:else}
    <div class="filters">
      <input bind:value={query} placeholder={$t('culture.search')} />
      <select bind:value={domain}>
        <option value="">{$t('culture.allDomains')}</option>
        {#each domains as entry (entry.slug)}<option value={entry.slug}>{label(entry)}</option>{/each}
      </select>
      <select bind:value={era}>
        <option value="">{$t('culture.allEras')}</option>
        {#each eras as entry (entry.slug)}<option value={entry.slug}>{label(entry)}</option>{/each}
      </select>
    </div>

    {#if loading}
      <Card><Skeleton lines={8} /></Card>
    {:else}
      <div class="layout">
        <Card title={tab === 'cards' ? $t('culture.cards') : $t('culture.quizzes')} subtitle={$t('culture.count', { count: tab === 'cards' ? visibleCards.length : visibleQuizzes.length })}>
          {#if (tab === 'cards' ? visibleCards : visibleQuizzes).length === 0}
            <EmptyState title={$t('culture.empty')} message={$t('culture.emptyHint')} />
          {:else}
            <ul class="item-list">
              {#each (tab === 'cards' ? visibleCards : visibleQuizzes) as item (item.uid)}
                <li>
                  <button class:active={selected?.uid === item.uid} on:click={() => open(item, item.kind)}>
                    <span class="line">
                      <strong>{item.title}</strong>
                      {#if item.status !== 'published'}<Badge tone="amber">{$t('school.pending')}</Badge>{/if}
                      {#if tab === 'cards' && progress[item.uid]?.status === 'completed'}<Badge tone="green">{$t('culture.read')}</Badge>{/if}
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

        <Card title={selected?.title ?? $t('culture.select')}>
          {#if detailLoading}
            <Skeleton lines={8} />
          {:else if !selected}
            <EmptyState title={$t('culture.select')} message={$t('culture.selectHint')} />
          {:else}
            <div class="detail">
              <div class="line">
                <Badge tone="blue">{selected.kind === 'quiz' ? $t('culture.quiz') : $t('culture.card')}</Badge>
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

              {#if selected.kind === 'quiz'}
                {#if quizResult}
                  <div class="score">
                    <strong>{$t('culture.score', { score: quizResult.score, max: quizResult.maxScore })}</strong>
                    <span>{$t('culture.scoreHint')}</span>
                  </div>
                  <ol class="correction">
                    {#each quizResult.correction as q, index (index)}
                      <li class:wrong={answers[index] !== q.correctIndex}>
                        <p>{q.prompt}</p>
                        <span>{$t('culture.answer')} : {q.choices[q.correctIndex] ?? '—'}</span>
                        {#if q.explanation}<span class="explain">{q.explanation}</span>{/if}
                      </li>
                    {/each}
                  </ol>
                {:else}
                  <ol class="questions">
                    {#each selected.questions ?? [] as q, index (q.id ?? index)}
                      <li>
                        <p>{q.prompt}</p>
                        <div class="choices">
                          {#each q.choices as choice, choiceIndex (choiceIndex)}
                            <label>
                              <input type="radio" name="q-{index}" checked={answers[index] === choiceIndex} on:change={() => { answers[index] = choiceIndex; answers = answers; }} />
                              {choice}
                            </label>
                          {/each}
                        </div>
                      </li>
                    {/each}
                  </ol>
                  {#if quizError}<p class="error">{quizError}</p>{/if}
                  <button class="btn primary" on:click={submitQuiz} disabled={submitting}>
                    {submitting ? $t('culture.submitting') : $t('culture.submit')}
                  </button>
                {/if}
              {:else if selectedProgress?.status !== 'completed'}
                <button class="btn primary" on:click={markRead}>{$t('culture.markRead')}</button>
              {/if}
              {#if canEdit}<button class="btn ghost" on:click={edit}>{$t('school.edit')}</button>{/if}
            </div>
          {/if}
        </Card>
      </div>
    {/if}
  {/if}
</section>

<SchoolEditor
  bind:open={editorOpen}
  kind={editorKind}
  item={editorItem}
  {taxonomy}
  on:close={() => (editorOpen = false)}
  on:saved={(event) => onSaved(event.detail)}
/>

<style>
  .culture {
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
  .tabs {
    display: flex;
    gap: 0.35rem;
    border-bottom: 1px solid var(--border);
  }
  .tabs button {
    border: 0;
    background: none;
    color: var(--muted);
    padding: 0.45rem 0.7rem;
    cursor: pointer;
    border-bottom: 2px solid transparent;
    font: inherit;
  }
  .tabs button.active {
    color: var(--text);
    border-bottom-color: var(--blue);
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
  .item-list {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
  }
  .item-list button {
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
  .item-list button:hover,
  .item-list button.active {
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
  .markdown :global(p) {
    margin: 0.45rem 0;
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
  .questions,
  .correction {
    margin: 0;
    padding-left: 1.2rem;
    display: flex;
    flex-direction: column;
    gap: 0.6rem;
    font-size: 0.85rem;
  }
  .questions p,
  .correction p {
    margin: 0 0 0.3rem;
    font-weight: 600;
  }
  .choices {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }
  .choices label {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    cursor: pointer;
  }
  .score {
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
    padding: 0.6rem 0.8rem;
    border-radius: var(--radius);
    background: color-mix(in srgb, var(--green) 12%, transparent);
    color: var(--green);
  }
  .score span {
    color: var(--muted);
    font-size: 0.78rem;
  }
  .correction li {
    color: var(--green);
  }
  .correction li.wrong {
    color: var(--red);
  }
  .correction span {
    display: block;
    color: var(--muted);
    font-size: 0.8rem;
  }
  .explain {
    margin-top: 0.2rem;
    font-style: italic;
  }
  .board {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.85rem;
  }
  .board th,
  .board td {
    text-align: left;
    padding: 0.4rem 0.5rem;
    border-bottom: 1px solid var(--border);
  }
  .board th {
    color: var(--muted);
    font-weight: 500;
  }
  .board tr.me {
    background: color-mix(in srgb, var(--blue) 10%, transparent);
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
