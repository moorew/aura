import { svelte } from '@sveltejs/vite-plugin-svelte';
import { defineConfig } from 'vitest/config';
import path from 'node:path';

// Vitest config kept separate from vite.config.ts so the SvelteKit plugin (which
// needs a full app context) doesn't interfere. We only need the Svelte compiler
// here so that `.svelte.ts` rune modules (e.g. lib/sync.svelte.ts) compile.
export default defineConfig({
  plugins: [svelte({ compilerOptions: { dev: true } })],
  resolve: {
    alias: {
      $lib: path.resolve(__dirname, './src/lib'),
    },
    conditions: ['browser'],
  },
  test: {
    // jsdom gives the sync engine the window/localStorage it reads directly.
    environment: 'jsdom',
    include: ['src/**/*.{test,spec}.{js,ts}'],
    globals: true,
  },
});
