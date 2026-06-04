import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vite';

const isTauri = !!process.env.TAURI_ENV_PLATFORM;

export default defineConfig({
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
