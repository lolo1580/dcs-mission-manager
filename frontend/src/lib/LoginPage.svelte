<script>
  import logo from '../assets/logo.png';
  import { login } from './session.js';
  import { t, lang, LANGUAGES, setLang } from './i18n.js';

  let username = '';
  let password = '';
  let busy = false;
  let error = '';
  let mode = 'login';
  let identifier = '';

  async function submit(event) {
    event.preventDefault();
    if (busy) return;
    if (mode === 'forgot') {
      busy = true;
      error = '';
      try {
        const response = await fetch('/api/auth/forgot', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ identifier: identifier.trim() }),
        });
        const body = await response.json().catch(() => ({}));
        if (!response.ok) throw new Error(body?.error ?? `${response.status}`);
        mode = 'sent';
      } catch (err) {
        error = err.message;
      } finally {
        busy = false;
      }
      return;
    }
    busy = true;
    error = '';
    try {
      await login(username.trim(), password);
    } catch (err) {
      error = $t('login.error', { detail: err.message });
    } finally {
      busy = false;
      password = '';
    }
  }
</script>

<div class="login">
  <form class="card" on:submit={submit}>
    <div class="brand">
      <img src={logo} alt="" aria-hidden="true" />
      <div>
        <strong>{$t('app.title')}</strong>
        <span>{$t('login.subtitle')}</span>
      </div>
    </div>

    <h1>{mode === 'login' ? $t('login.title') : $t('login.forgotTitle')}</h1>
    <p class="hint">{mode === 'login' ? $t('login.hint') : $t('login.forgotHint')}</p>

    {#if mode === 'login'}
      <label class="field">
        <span>{$t('login.username')}</span>
        <input bind:value={username} autocomplete="username" autocapitalize="none" spellcheck="false" required />
      </label>
      <label class="field">
        <span>{$t('login.password')}</span>
        <input type="password" bind:value={password} autocomplete="current-password" required />
      </label>
      <button class="link" type="button" on:click={() => { mode = 'forgot'; error = ''; identifier = username; }}>
        {$t('login.forgot')}
      </button>
    {:else}
      <label class="field">
        <span>{$t('login.identifier')}</span>
        <input bind:value={identifier} autocomplete="username" autocapitalize="none" spellcheck="false" required />
      </label>
      {#if mode === 'sent'}
        <p class="info">{$t('login.resetSent')}</p>
        <button class="link" type="button" on:click={() => { mode = 'login'; error = ''; }}>
          {$t('login.backToLogin')}
        </button>
      {:else}
        <button class="link" type="button" on:click={() => { mode = 'login'; error = ''; }}>
          {$t('login.backToLogin')}
        </button>
      {/if}
    {/if}

    {#if error}<p class="error">{error}</p>{/if}

    {#if mode !== 'sent'}
      <button class="btn primary submit" type="submit" disabled={busy}>
        {busy ? $t('login.busy') : (mode === 'login' ? $t('login.submit') : $t('login.sendReset'))}
      </button>
    {/if}

    <div class="lang">
      <select value={$lang} on:change={(event) => setLang(event.currentTarget.value)} aria-label="Language">
        {#each LANGUAGES as language (language.id)}
          <option value={language.id}>{language.label}</option>
        {/each}
      </select>
    </div>
  </form>
</div>

<style>
  .login {
    display: grid;
    place-items: center;
    min-height: 100%;
    padding: 2rem 1rem;
    background:
      radial-gradient(circle at 20% 0%, rgba(76, 154, 255, 0.12), transparent 45%),
      radial-gradient(circle at 80% 100%, rgba(63, 208, 122, 0.08), transparent 45%),
      var(--bg);
  }

  .card {
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
    width: min(380px, 100%);
    padding: 1.4rem;
    background: var(--panel);
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-lg);
    box-shadow: var(--shadow);
  }

  .brand {
    display: flex;
    align-items: center;
    gap: 0.6rem;
  }

  .brand img {
    width: 38px;
    height: 38px;
    border-radius: 50%;
  }

  .brand div {
    display: flex;
    flex-direction: column;
    line-height: 1.2;
  }

  .brand strong {
    font-size: 0.95rem;
  }

  .brand span {
    font-size: 0.72rem;
    color: var(--muted);
  }

  h1 {
    margin: 0.5rem 0 0;
    font-size: 1.25rem;
  }

  .hint {
    margin: 0;
    color: var(--muted);
    font-size: 0.8rem;
    line-height: 1.45;
  }

  .submit {
    margin-top: 0.25rem;
    padding: 0.55rem 0.8rem;
    font-size: 0.86rem;
  }

  .error {
    margin: 0;
    padding: 0.45rem 0.6rem;
    color: var(--red);
    background: var(--red-soft);
    border: 1px solid color-mix(in srgb, var(--red) 40%, transparent);
    border-radius: var(--radius-sm);
    font-size: 0.78rem;
  }

  .info {
    margin: 0;
    padding: 0.45rem 0.6rem;
    color: var(--green);
    background: var(--green-soft);
    border: 1px solid color-mix(in srgb, var(--green) 40%, transparent);
    border-radius: var(--radius-sm);
    font-size: 0.78rem;
    line-height: 1.4;
  }

  .link {
    align-self: flex-start;
    padding: 0;
    color: var(--blue);
    background: none;
    border: none;
    font-size: 0.76rem;
    cursor: pointer;
  }

  .link:hover {
    text-decoration: underline;
  }

  .lang {
    display: flex;
    justify-content: flex-end;
  }

  .lang select {
    padding: 0.25rem 0.4rem;
    color: var(--muted);
    background: transparent;
    border: 1px solid transparent;
    border-radius: var(--radius-sm);
    font-size: 0.75rem;
    cursor: pointer;
  }

  .lang select:hover {
    border-color: var(--border);
  }
</style>
