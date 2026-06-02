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

  <!-- ── Sidebar ────────────────────────────────────────────────────────── -->
  <aside class="flex w-48 shrink-0 flex-col border-r border-gray-100 bg-white
                dark:border-gray-800/60 dark:bg-gray-900">

    <!-- Logo -->
    <div class="flex items-center gap-2.5 px-5 py-5">
      <div class="flex h-7 w-7 shrink-0 items-center justify-center rounded-lg bg-blue-500 shadow-sm shadow-blue-200 dark:shadow-none">
        <span class="text-xs font-bold text-white">S</span>
      </div>
      <span class="text-sm font-bold tracking-tight text-gray-900 dark:text-gray-50">Sempa</span>
    </div>

    <!-- Nav links -->
    <nav class="flex flex-1 flex-col gap-0.5 px-3 pb-3">

      {#snippet navLink(href: string, label: string, icon: import('svelte').Snippet)}
        {@const active = isActive(href)}
        <a {href}
           class="group flex items-center gap-2.5 rounded-lg px-3 py-2 text-sm transition-colors
                  {active
                    ? 'bg-blue-50 text-blue-600 font-semibold dark:bg-blue-950/60 dark:text-blue-400'
                    : 'text-gray-500 hover:bg-gray-50 hover:text-gray-800 dark:text-gray-500 dark:hover:bg-gray-800 dark:hover:text-gray-200'}">
          <span class="shrink-0 {active ? 'text-blue-500 dark:text-blue-400' : 'text-gray-400 group-hover:text-gray-600 dark:group-hover:text-gray-300'}">
            {@render icon()}
          </span>
          {label}
        </a>
      {/snippet}

      {#snippet iconToday()}
        <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24">
          <circle cx="12" cy="12" r="4"/><path stroke-linecap="round" d="M12 2v2m0 16v2M4.93 4.93l1.41 1.41m11.32 11.32 1.41 1.41M2 12h2m16 0h2M4.93 19.07l1.41-1.41M18.66 5.34l1.41-1.41"/>
        </svg>
      {/snippet}
      {@render navLink(`/day/${todayDate}`, 'Today', iconToday)}

      {#snippet iconWeek()}
        <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24">
          <rect x="3" y="4" width="18" height="18" rx="2"/><path stroke-linecap="round" d="M16 2v4M8 2v4M3 10h18"/>
        </svg>
      {/snippet}
      {@render navLink(`/week/${thisWeek}`, 'This Week', iconWeek)}

      {#snippet iconPlan()}
        <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24">
          <path stroke-linecap="round" d="M9 5H7a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V7a2 2 0 0 0-2-2h-2M9 5a2 2 0 0 0 2 2h2a2 2 0 0 0 2-2M9 5a2 2 0 0 1 2-2h2a2 2 0 0 1 2 2m-6 9 2 2 4-4"/>
        </svg>
      {/snippet}
      {@render navLink(`/plan/${todayDate}`, 'Plan Day', iconPlan)}

      {#snippet iconEmail()}
        <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24">
          <path stroke-linecap="round" d="M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"/>
        </svg>
      {/snippet}
      {@render navLink('/email', 'Inbox', iconEmail)}

      {#snippet iconShutdown()}
        <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24">
          <path stroke-linecap="round" d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"/>
        </svg>
      {/snippet}
      {@render navLink(`/shutdown/${todayDate}`, 'Shutdown', iconShutdown)}

      <!-- Pomodoro in-progress widget -->
      {#if pomodoro.taskId}
        <div class="mt-2 rounded-xl border border-amber-200/70 bg-amber-50 px-3 py-2.5
                    dark:border-amber-800/40 dark:bg-amber-950/40">
          <p class="truncate text-[10px] font-semibold uppercase tracking-wider text-amber-600 dark:text-amber-500">
            {pomodoro.phaseLabel}
          </p>
          <p class="font-mono text-xl font-bold text-amber-600 dark:text-amber-400">
            {pomodoro.display}
          </p>
        </div>
      {/if}

      <!-- Bottom section -->
      <div class="mt-auto flex flex-col gap-0.5 border-t border-gray-100 pt-3 dark:border-gray-800/60">

        {#snippet iconSettings()}
          <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24">
            <path stroke-linecap="round" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 0 0 2.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 0 0 1.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 0 0-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 0 0-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 0 0-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 0 0-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 0 0 1.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"/><circle cx="12" cy="12" r="3"/>
          </svg>
        {/snippet}
        {@render navLink('/settings/accounts', 'Settings', iconSettings)}

        <!-- Dark mode toggle -->
        <button onclick={theme.toggle}
                class="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-sm text-gray-500
                       transition-colors hover:bg-gray-50 hover:text-gray-800
                       dark:text-gray-500 dark:hover:bg-gray-800 dark:hover:text-gray-200">
          <span class="shrink-0 text-gray-400">
            {#if theme.dark}
              <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24">
                <circle cx="12" cy="12" r="4"/><path stroke-linecap="round" d="M12 2v2m0 16v2M4.93 4.93l1.41 1.41m11.32 11.32 1.41 1.41M2 12h2m16 0h2M4.93 19.07l1.41-1.41M18.66 5.34l1.41-1.41"/>
              </svg>
            {:else}
              <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24">
                <path stroke-linecap="round" d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"/>
              </svg>
            {/if}
          </span>
          {theme.dark ? 'Light mode' : 'Dark mode'}
        </button>
      </div>
    </nav>
  </aside>

  <!-- ── Main content ───────────────────────────────────────────────────── -->
  <div class="flex-1 overflow-auto">
    {@render children()}
  </div>
</div>

{#if pomodoro.taskId}
  <PomodoroTimer />
{/if}
{/if}
