<script>
  import { chat } from './session.js';
  import { t, tNow } from './i18n.js';

  let draft = '';
  let sending = false;
  let error = '';

  async function submit() {
    const message = draft.trim();
    if (!message) return;
    sending = true;
    error = '';
    try {
      const res = await fetch('/api/chat', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ message }),
      });
      if (!res.ok) {
        // 503 means DCS is not connected: a normal state, not a failure.
        if (res.status === 503) {
          error = tNow('chat.notConnected');
          return;
        }
        const body = await res.json().catch(() => ({}));
        error = body.error ?? tNow('chat.refused', { status: res.status });
        return;
      }
      draft = '';
    } catch {
      error = tNow('chat.unreachable');
    } finally {
      sending = false;
    }
  }

  function timeOf(c) {
    return new Date(c.realTs).toLocaleTimeString();
  }
</script>

<section class="chat">
  <h2>
    {$t('chat.title')} <span class="count">{$chat.length}</span>
  </h2>

  <ul>
    {#each $chat.slice(-200) as c (`${c.id}`)}
      <li>
        <span class="t">{timeOf(c)}</span>
        <span class="from">{c.from || $t('chat.system')}</span>
        <span class="msg">{c.message}</span>
      </li>
    {/each}
    {#if $chat.length === 0}
      <li class="empty">{$t('chat.none')}</li>
    {/if}
  </ul>

  <form on:submit|preventDefault={submit}>
    <input bind:value={draft} placeholder={$t('chat.placeholder')} disabled={sending} />
    <button type="submit" disabled={sending || !draft.trim()}>{$t('chat.send')}</button>
  </form>
  {#if error}<p class="error">{error}</p>{/if}
</section>

<style>
  .chat {
    display: flex;
    flex-direction: column;
    min-height: 0;
    padding: 0.9rem 1rem;
  }

  h2 {
    margin: 0 0 0.6rem;
    font-size: 0.8rem;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--muted);
    display: flex;
    justify-content: space-between;
  }

  .count {
    color: var(--text);
    font-variant-numeric: tabular-nums;
  }

  ul {
    list-style: none;
    margin: 0 0 0.5rem;
    padding: 0;
    overflow-y: auto;
    min-height: 80px;
    max-height: 180px;
    font-size: 0.76rem;
  }

  li {
    display: flex;
    gap: 0.4rem;
    padding: 0.15rem 0;
    border-top: 1px solid var(--border);
  }

  li:first-child {
    border-top: none;
  }

  .t {
    color: var(--muted);
    font-variant-numeric: tabular-nums;
    flex: none;
  }

  .from {
    color: var(--blue);
    flex: none;
    max-width: 90px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .msg {
    overflow-wrap: anywhere;
  }

  .empty {
    color: var(--muted);
    border-top: none;
  }

  form {
    display: flex;
    gap: 0.35rem;
  }

  input {
    flex: 1;
    min-width: 0;
    padding: 0.35rem 0.5rem;
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 6px;
    color: var(--text);
    font-size: 0.78rem;
  }

  input:focus {
    outline: none;
    border-color: var(--blue);
  }

  button {
    padding: 0.35rem 0.6rem;
    font-size: 0.76rem;
    color: var(--text);
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 6px;
    cursor: pointer;
  }

  button:hover:not(:disabled) {
    border-color: var(--blue);
  }

  button:disabled {
    opacity: 0.5;
    cursor: default;
  }

  .error {
    margin: 0.4rem 0 0;
    color: #f0b429;
    font-size: 0.72rem;
  }
</style>
