<script>
  import { toasts, dismissToast } from '../toast.js';

  const COLORS = {
    success: 'var(--green)',
    error: 'var(--red)',
    info: 'var(--blue)',
  };
</script>

<div class="toasts" aria-live="polite" aria-atomic="false">
  {#each $toasts as toast (toast.id)}
    <button
      class="toast"
      style="--toast-color:{COLORS[toast.tone] ?? COLORS.info}"
      on:click={() => dismissToast(toast.id)}
    >
      {toast.message}
    </button>
  {/each}
</div>

<style>
  .toasts {
    position: fixed;
    right: 1rem;
    bottom: 1rem;
    z-index: 60;
    display: flex;
    flex-direction: column;
    gap: 0.45rem;
    max-width: min(380px, calc(100vw - 2rem));
    pointer-events: none;
  }

  .toast {
    pointer-events: auto;
    padding: 0.55rem 0.75rem;
    text-align: left;
    color: var(--text);
    background: var(--panel-2);
    border: 1px solid var(--border-strong);
    border-left: 3px solid var(--toast-color);
    border-radius: var(--radius);
    box-shadow: var(--shadow);
    font-size: 0.8rem;
    cursor: pointer;
  }
</style>
