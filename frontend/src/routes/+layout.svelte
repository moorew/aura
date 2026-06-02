<script lang="ts">
  import '../app.css';
  import { page } from '$app/stores';
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { today, weekStart } from '$lib/utils';
  import { pomodoro } from '$lib/stores/pomodoro.svelte';
  import { theme } from '$lib/stores/theme.svelte';
  import { tagStore } from '$lib/stores/tags.svelte';
  import { api } from '$lib/api';
  import PomodoroTimer from '$lib/components/PomodoroTimer.svelte';
  import type { Snippet } from 'svelte';

  let { children }: { children: Snippet } = $props();

  const todayDate = today();
  const thisWeek  = weekStart(todayDate);

  // Track whether we're on the login page to skip auth check and hide sidebar
  let isLoginPage = $derived($page.url.pathname === '/login');

  onMount(async () => {
    theme.init();
    if (!isLoginPage) {
      tagStore.load();
      try {
        const me = await api.auth.me();
        if (!me.authenticated) {
          goto('/login?redirect=' + encodeURIComponent($page.url.pathname), { replaceState: true });
        }
      } catch {
        goto('/login?redirect=' + encodeURIComponent($page.url.pathname), { replaceState: true });
      }
    }
  });

  function isActive(prefix: string): boolean {
    return $page.url.pathname.startsWith(prefix);
  }
</script>

{#if isLoginPage}
  {@render children()}
{:else}
<div class="flex h-screen overflow-hidden bg-gray-50 dark:bg-gray-950">
  <!-- ── Sidebar ─────────────────────────────────────────────────────── -->
  <aside class="flex w-44 shrink-0 flex-col border-r border-gray-200 bg-white dark:border-gray-800 dark:bg-gray-900">
    <!-- Logo -->
    <div class="flex items-center gap-2.5 border-b border-gray-100 px-4 py-4 dark:border-gray-800">
      <div class="flex h-7 w-7 shrink-0 items-center justify-center rounded-lg bg-blue-500">
        <span class="text-xs font-bold text-white">S</span>
      </div>
      <span class="text-sm font-semibold tracking-tight text-gray-900 dark:text-gray-50">Sempa</span>
    </div>

    <!-- Nav -->
    <nav class="flex flex-1 flex-col gap-0.5 px-2 py-3">
      <a href="/day/{todayDate}"
         class="flex items-center gap-2.5 rounded-lg px-3 py-2 text-sm transition-colors
                {isActive('/day') ? 'bg-blue-50 font-medium text-blue-600 dark:bg-blue-950 dark:text-blue-400' : 'text-gray-600 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-gray-800'}">
        <svg class="h-4 w-4 shrink-0" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24">
          <circle cx="12" cy="12" r="4"/><path stroke-linecap="round" d="M12 2v2m0 16v2M4.93 4.93l1.41 1.41m11.32 11.32 1.41 1.41M2 12h2m16 0h2M4.93 19.07l1.41-1.41M18.66 5.34l1.41-1.41"/>
        </svg>
        Today
      </a>

      <a href="/week/{thisWeek}"
         class="flex items-center gap-2.5 rounded-lg px-3 py-2 text-sm transition-colors
                {isActive('/week') ? 'bg-blue-50 font-medium text-blue-600 dark:bg-blue-950 dark:text-blue-400' : 'text-gray-600 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-gray-800'}">
        <svg class="h-4 w-4 shrink-0" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24">
          <rect x="3" y="4" width="18" height="18" rx="2"/><path stroke-linecap="round" d="M16 2v4M8 2v4M3 10h18"/>
        </svg>
        This Week
      </a>

      <a href="/plan/{todayDate}"
         class="flex items-center gap-2.5 rounded-lg px-3 py-2 text-sm transition-colors
                {isActive('/plan') ? 'bg-blue-50 font-medium text-blue-600 dark:bg-blue-950 dark:text-blue-400' : 'text-gray-600 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-gray-800'}">
        <svg class="h-4 w-4 shrink-0" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24">
          <path stroke-linecap="round" d="M9 5H7a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V7a2 2 0 0 0-2-2h-2M9 5a2 2 0 0 0 2 2h2a2 2 0 0 0 2-2M9 5a2 2 0 0 1 2-2h2a2 2 0 0 1 2 2m-6 9 2 2 4-4"/>
        </svg>
        Plan Day
      </a>

      <a href="/shutdown/{todayDate}"
         class="flex items-center gap-2.5 rounded-lg px-3 py-2 text-sm transition-colors
                {isActive('/shutdown') ? 'bg-blue-50 font-medium text-blue-600 dark:bg-blue-950 dark:text-blue-400' : 'text-gray-600 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-gray-800'}">
        <svg class="h-4 w-4 shrink-0" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24">
          <path stroke-linecap="round" d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"/>
        </svg>
        Shutdown
      </a>

      <!-- Spacer + Pomodoro mini display -->
      {#if pomodoro.taskId}
        <div class="mt-2 border-t border-gray-100 pt-2 dark:border-gray-800">
          <div class="rounded-lg bg-amber-50 px-3 py-2 dark:bg-amber-950">
            <p class="truncate text-xs font-medium text-amber-700 dark:text-amber-400">{pomodoro.phaseLabel}</p>
            <p class="font-mono text-lg font-bold text-amber-600 dark:text-amber-400">{pomodoro.display}</p>
          </div>
        </div>
      {/if}

      <!-- Settings + dark toggle pinned to bottom -->
      <div class="mt-auto border-t border-gray-100 pt-2 dark:border-gray-800">
        <a href="/settings/accounts"
           class="flex items-center gap-2.5 rounded-lg px-3 py-2 text-sm transition-colors
                  {isActive('/settings') ? 'bg-blue-50 font-medium text-blue-600 dark:bg-blue-950 dark:text-blue-400' : 'text-gray-600 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-gray-800'}">
          <svg class="h-4 w-4 shrink-0" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24">
            <path stroke-linecap="round" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 0 0 2.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 0 0 1.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 0 0-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 0 0-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 0 0-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 0 0-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 0 0 1.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"/><circle cx="12" cy="12" r="3"/>
          </svg>
          Settings
        </a>

        <!-- Dark mode toggle -->
        <button onclick={theme.toggle}
                class="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-sm text-gray-600 transition-colors hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-gray-800"
                title="Toggle dark mode">
          {#if theme.dark}
            <svg class="h-4 w-4 shrink-0" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24">
              <circle cx="12" cy="12" r="4"/><path stroke-linecap="round" d="M12 2v2m0 16v2M4.93 4.93l1.41 1.41m11.32 11.32 1.41 1.41M2 12h2m16 0h2M4.93 19.07l1.41-1.41M18.66 5.34l1.41-1.41"/>
            </svg>
            Light mode
          {:else}
            <svg class="h-4 w-4 shrink-0" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24">
              <path stroke-linecap="round" d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"/>
            </svg>
            Dark mode
          {/if}
        </button>
      </div>
    </nav>
  </aside>

  <!-- ── Main content ─────────────────────────────────────────────────── -->
  <div class="flex-1 overflow-auto">
    {@render children()}
  </div>
</div>

<!-- ── Floating Pomodoro timer ────────────────────────────────────────── -->
{#if pomodoro.taskId}
  <PomodoroTimer />
{/if}
{/if}
