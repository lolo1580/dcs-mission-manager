<script>
  export let lines = 3;
</script>

<div class="skeleton" aria-hidden="true">
  {#each Array(lines) as _, i}
    <span style="width:{i === lines - 1 ? '55%' : '100%'}"></span>
  {/each}
</div>

<style>
  .skeleton {
    display: flex;
    flex-direction: column;
    gap: 0.45rem;
    padding: 0.5rem 0;
  }

  span {
    height: 10px;
    border-radius: 5px;
    background: linear-gradient(90deg, var(--bg-elevated), var(--border), var(--bg-elevated));
    background-size: 200% 100%;
    animation: shimmer 1.4s ease-in-out infinite;
  }

  @keyframes shimmer {
    0% { background-position: 200% 0; }
    100% { background-position: -200% 0; }
  }
</style>
