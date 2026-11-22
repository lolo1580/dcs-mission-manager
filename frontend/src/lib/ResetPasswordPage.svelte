<script>
  import logo from '../assets/logo.png';
  import { t } from './i18n.js';

  const params = typeof window !== 'undefined' ? new URLSearchParams(window.location.search) : new URLSearchParams();
  const hashParams = typeof window !== 'undefined'
    ? new URLSearchParams(window.location.hash.replace(/^#/, ''))
    : new URLSearchParams();
  const token = hashParams.get('token') ?? params.get('token') ?? '';

  let password = '';
  let confirmation = '';
  let busy = false;
  let error = '';
  let done = false;

  async function submit(event) {
    event.preventDefault();
    if (busy) return;
    error = '';
    if (password !== confirmation) {
      error = $t('profile.passwordMismatch');
      return;
    }
    busy = true;
    try {
      const response = await fetch('/api/auth/reset', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ token, newPassword: password }),
      });
      const body = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(body?.error ?? `${response.status}`);
      done = true;
    } catch (err) {
      error = err.message;
    } finally {
      busy = false;
    }
  }

  function signIn() {
    window.location.href = '/squadron';
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

    <h1>{$t('reset.title')}</h1>

    {#if done}
      <p class="info">{$t('reset.success')}</p>
      <button class="btn primary submit" type="button" on:click={signIn}>{$t('reset.signIn')}</button>
    {:else if !token}
      <p class="error">{$t('reset.invalid')}</p>
      <button class="btn submit" type="button" on:click={signIn}>{$t('reset.signIn')}</button>
    {:else}
      <p class="hint">{$t('reset.hint')}</p>
      <label class="field">
        <span>{$t('reset.newPassword')}</span>
        <input type="password" bind:value={password} minlength="10" autocomplete="new-password" required />
      </label>
      <label class="field">
        <span>{$t('reset.confirm')}</span>
        <input type="password" bind:value={confirmation} minlength="10" autocomplete="new-password" required />
      </label>
      {#if error}<p class="error">{error}</p>{/if}
      <button class="btn primary submit" type="submit" disabled={busy}>
        {busy ? $t('login.busy') : $t('reset.submit')}
      </button>
    {/if}
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

  .error,
  .info {
    margin: 0;
    padding: 0.45rem 0.6rem;
    border-radius: var(--radius-sm);
    font-size: 0.78rem;
    line-height: 1.4;
  }

  .error {
    color: var(--red);
    background: var(--red-soft);
    border: 1px solid color-mix(in srgb, var(--red) 40%, transparent);
  }

  .info {
    color: var(--green);
    background: var(--green-soft);
    border: 1px solid color-mix(in srgb, var(--green) 40%, transparent);
  }
</style>
