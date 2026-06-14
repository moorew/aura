import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vite';
import { readFileSync } from 'node:fs';

const isTauri = !!process.env.TAURI_ENV_PLATFORM;

// Single source of truth for the app version (used by the in-app updater to
// compare against the latest GitHub release). Read from package.json at build
// time so it never drifts from the published version.
const pkg = JSON.parse(readFileSync(new URL('./package.json', import.meta.url), 'utf-8'));

export default defineConfig({
  define: {
    __APP_VERSION__: JSON.stringify(pkg.version),
  },

  plugins: [tailwindcss(), sveltekit()],

  // Tauri expects a fixed port for the dev server
  server: {
    port: 5173,
    strictPort: true,
    // Tauri uses localhost on Windows
    host: isTauri ? '0.0.0.0' : undefined,
  },

  // Env variables prefixed with TAURI_ are exposed to the frontend
  envPrefix: ['VITE_', 'TAURI_ENV_'],

  build: {
    // Tauri uses Chromium on Windows via WebView2 — target modern ES
    target: isTauri ? 'chrome105' : undefined,
  },
});
