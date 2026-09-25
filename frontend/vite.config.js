import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';

// The build output is written into the Go package that embeds it, so that
// `go build` produces a single self-contained binary. It goes to `dist/`
// (git-ignored) rather than `web/` (the committed fallback page), so that
// build artifacts never end up in version control.
export default defineConfig({
  plugins: [svelte()],
  build: {
    outDir: '../backend/internal/api/dist',
    // Never let Vite wipe the directory: it holds a committed `.gitkeep` that
    // `//go:embed` relies on. Stale output is removed by scripts/clean-dist.mjs.
    emptyOutDir: false,
  },
  server: {
    proxy: {
      '/api': 'http://localhost:8080',
    },
  },
});
