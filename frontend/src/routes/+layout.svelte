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

  // Lucide icons
  import {
    Sun, Calendar, ClipboardCheck, Inbox, Moon, Settings,
    ChevronLeft, ChevronRight, Plus, RefreshCw, X, Check,
    Target, Timer, LayoutDashboard, Palette,
  } from 'lucide-svelte';

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
      <div class="flex h-7 w-7 shrink-0 items-center justify-center rounded-lg shadow-sm"
           style="background: var(--a500);">
        <span class="text-xs font-bold text-white">S</span>
      </div>
      <span class="text-sm font-bold tracking-tight text-gray-900 dark:text-gray-50">Sempa</span>
    </div>

    <!-- Nav -->
    <nav class="flex flex-1 flex-col gap-0.5 px-3 pb-3">

      {#snippet navItem(href: string, label: string, Icon: any)}
        {@const active = isActive(href)}
        <a {href}
           class="group flex items-center gap-2.5 rounded-lg px-3 py-2 text-sm transition-colors
                  {active ? 'font-semibold' : 'text-gray-500 hover:bg-gray-50 hover:text-gray-800 dark:text-gray-500 dark:hover:bg-gray-800 dark:hover:text-gray-200'}"
           style={active ? `background:var(--a50); color:var(--a600);` : ''}>
          <span class="shrink-0 transition-colors"
                style={active ? `color:var(--a500)` : ''}>
            <Icon size={16} strokeWidth={active ? 2.25 : 1.75} />
          </span>
          {label}
        </a>
      {/snippet}

      {@render navItem(`/day/${todayDate}`, 'Today', LayoutDashboard)}
      {@render navItem(`/week/${thisWeek}`, 'This Week', Calendar)}
      {@render navItem(`/plan/${todayDate}`, 'Plan Day', ClipboardCheck)}
      {@render navItem('/email', 'Email', Inbox)}
      {@render navItem(`/shutdown/${todayDate}`, 'Shutdown', Moon)}

      <!-- Pomodoro in-progress -->
      {#if pomodoro.taskId}
        <div class="mt-2 rounded-xl border px-3 py-2.5"
             style="border-color:var(--a200); background:var(--a50); color:var(--a700);">
          <p class="truncate text-[10px] font-semibold uppercase tracking-wider opacity-70">
            {pomodoro.phaseLabel}
          </p>
          <p class="font-mono text-xl font-bold">{pomodoro.display}</p>
        </div>
      {/if}

      <!-- Bottom section -->
      <div class="mt-auto flex flex-col gap-0.5 border-t border-gray-100 pt-3 dark:border-gray-800/60">
        {@render navItem('/settings/accounts', 'Settings', Settings)}

        <button onclick={theme.toggle}
                class="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-sm text-gray-500
                       transition-colors hover:bg-gray-50 hover:text-gray-800
                       dark:text-gray-500 dark:hover:bg-gray-800 dark:hover:text-gray-200">
          <span class="shrink-0">
            {#if theme.dark}
              <Sun size={16} strokeWidth={1.75} />
            {:else}
              <Moon size={16} strokeWidth={1.75} />
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
