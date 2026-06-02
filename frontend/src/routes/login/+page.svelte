<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { page } from '$app/stores';
  import { api } from '$lib/api';

  let username = $state('');
  let password = $state('');
  let loading = $state(false);
  let error = $state('');

  onMount(async () => {
    // Already logged in? Go home.
    try {
      const me = await api.auth.me();
      if (me.authenticated) {
        const redirect = $page.url.searchParams.get('redirect') ?? '/';
        goto(redirect, { replaceState: true });
      }
    } catch { /* not logged in */ }
  });

  async function submit() {
    if (!username.trim() || !password) return;
    loading = true; error = '';
    try {
      await api.auth.login(username.trim(), password);
      const redirect = $page.url.searchParams.get('redirect') ?? '/';
      goto(redirect, { replaceState: true });
    } catch (e) {
      error = 'Invalid username or password.';
    } finally {
      loading = false;
    }
  }
</script>

<svelte:head><title>Sign in — Aura</title></svelte:head>

<div class="flex min-h-screen items-center justify-center bg-gray-50 dark:bg-gray-950 px-4">
  <div class="w-full max-w-sm">
    <!-- Logo -->
    <div class="mb-8 flex flex-col items-center gap-3">
      <div class="flex h-12 w-12 items-center justify-center rounded-2xl bg-blue-500 shadow-lg">
        <span class="text-xl font-bold text-white">A</span>
      </div>
      <h1 class="text-xl font-semibold text-gray-900 dark:text-gray-50">Sign in to Aura</h1>
    </div>

    <form onsubmit={(e) => { e.preventDefault(); submit(); }}
          class="rounded-2xl border border-gray-200 bg-white p-6 shadow-sm dark:border-gray-800 dark:bg-gray-900 space-y-4">
      <div>
        <label for="username" class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">Username</label>
        <input id="username" type="text" bind:value={username} autocomplete="username"
               autofocus
               class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm outline-none
                      focus:border-blue-500 focus:ring-2 focus:ring-blue-100
                      dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100 dark:focus:ring-blue-900" />
      </div>
      <div>
        <label for="password" class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">Password</label>
        <input id="password" type="password" bind:value={password} autocomplete="current-password"
               class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm outline-none
                      focus:border-blue-500 focus:ring-2 focus:ring-blue-100
                      dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100 dark:focus:ring-blue-900" />
      </div>

      {#if error}
        <p class="text-sm text-red-600 dark:text-red-400">{error}</p>
      {/if}

      <button type="submit" disabled={loading || !username || !password}
              class="w-full rounded-lg bg-blue-500 py-2.5 text-sm font-medium text-white
                     hover:bg-blue-600 disabled:opacity-40 transition-colors">
        {loading ? 'Signing in…' : 'Sign in'}
      </button>
    </form>
  </div>
</div>
