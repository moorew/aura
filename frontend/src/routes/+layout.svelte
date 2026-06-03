<script lang="ts">
  import '../app.css';
  import { page } from '$app/stores';
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { today, weekStart, offsetDate } from '$lib/utils';
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

  let isLoginPage      = $derived(($page.url.pathname as string) === '/login');
  let shortcutsOpen    = $state(false);
  let userEmail        = $state<string | undefined>(undefined);

  const SHORTCUT_HELP = [
    { key: 'n',   desc: 'New task (on day view)' },
    { key: 't',   desc: 'Go to today' },
    { key: 'j',   desc: 'Previous week' },
    { key: 'k',   desc: 'Next week' },
    { key: '?',   desc: 'Show keyboard shortcuts' },
    { key: 'Esc', desc: 'Close this dialog' },
  ];

  function handleKeydown(e: KeyboardEvent) {
    if (isLoginPage) return;
    const tgt = e.target as HTMLElement;
    if (tgt.tagName === 'INPUT' || tgt.tagName === 'TEXTAREA' || tgt.isContentEditable) return;
    if (e.metaKey || e.ctrlKey || e.altKey) return;

    if (e.key === 'Escape') { shortcutsOpen = false; return; }
    if (shortcutsOpen) return;

    const path = $page.url.pathname;
    const dayMatch = path.match(/^\/day\/(\d{4}-\d{2}-\d{2})/);
    const curDate = dayMatch?.[1] ?? todayDate;
    const curWs   = weekStart(curDate);

    switch (e.key) {
      case 't': e.preventDefault(); goto(`/day/${todayDate}`); break;
      case 'j': e.preventDefault(); goto(`/day/${offsetDate(curWs, -7)}`); break;
      case 'k': e.preventDefault(); goto(`/day/${offsetDate(curWs, 7)}`); break;
      case '?': e.preventDefault(); shortcutsOpen = true; break;
    }
  }

  onMount(async () => {
    theme.init();
    if (!isLoginPage) {
      tagStore.load();
      try {
        const me = await api.auth.me();
        if (!me.authenticated) {
          goto('/login?redirect=' + encodeURIComponent($page.url.pathname), { replaceState: true });
        } else {
          userEmail = me.email;
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

<svelte:window onkeydown={handleKeydown} />

{#if isLoginPage}
  {@render children()}
{:else}
<div class="flex h-screen overflow-hidden" style="background: var(--sempa-bg-main);">

  <!-- ── Sidebar ────────────────────────────────────────────────────────── -->
  <aside class="flex w-48 shrink-0 flex-col"
         style="background: var(--sempa-bg-nav); border-right: 1px solid var(--sempa-border);">

    <!-- Logo (Cradle mark) -->
    <div class="flex items-center gap-2 px-4 py-5" style="color: var(--sempa-accent);">
      <svg width="26" height="26" viewBox="0 0 100 100" fill="none" aria-hidden="true">
        <path d="M22,40 a28,28 0 0 0 56,0"
          stroke="currentColor" stroke-width="9"
          stroke-linecap="round" stroke-linejoin="round"/>
        <circle cx="50" cy="35" r="7.5" fill="currentColor"/>
      </svg>
      <span style="font-family: 'Plus Jakarta Sans', sans-serif; font-weight: 500;
                   font-size: 18px; letter-spacing: -0.02em; color: var(--sempa-text);">sempa</span>
    </div>

    <!-- Nav -->
    <nav class="flex flex-1 flex-col gap-0.5 px-3 pb-3">

      {#snippet navItem(href: string, label: string, Icon: any)}
        {@const active = isActive(href)}
        <a {href}
           class="group flex items-center gap-2.5 rounded-lg px-3 py-2 text-[13.5px] tracking-[-0.01em] transition-colors"
           style={active
             ? `background: var(--sempa-accent-bg); color: var(--sempa-accent); font-weight: 600;`
             : `color: var(--sempa-text-soft);`}
           onmouseenter={(e) => { if (!active) (e.currentTarget as HTMLElement).style.background = 'rgba(0,0,0,0.04)'; }}
           onmouseleave={(e) => { if (!active) (e.currentTarget as HTMLElement).style.background = ''; }}>
          <span class="shrink-0 transition-colors"
                style={active ? `color: var(--sempa-accent)` : ''}>
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
             style="border-color: var(--sempa-accent-bg); background: var(--sempa-accent-bg); color: var(--sempa-accent);">
          <p class="truncate text-[10px] font-semibold uppercase tracking-wider opacity-70">
            {pomodoro.phaseLabel}
          </p>
          <p class="font-mono text-xl font-bold">{pomodoro.display}</p>
        </div>
      {/if}

      <!-- Bottom section -->
      <div class="mt-auto flex flex-col gap-0.5 pt-3"
           style="border-top: 1px solid var(--sempa-border);">
        {@render navItem('/settings/accounts', 'Settings', Settings)}

        <button onclick={theme.toggle}
                class="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-[13.5px] tracking-[-0.01em] transition-colors"
                style="color: var(--sempa-text-soft);"
                onmouseenter={(e) => (e.currentTarget as HTMLElement).style.background = 'rgba(0,0,0,0.04)'}
                onmouseleave={(e) => (e.currentTarget as HTMLElement).style.background = ''}>
          <span class="shrink-0">
            {#if theme.dark}
              <Sun size={16} strokeWidth={1.75} />
            {:else}
              <Moon size={16} strokeWidth={1.75} />
            {/if}
          </span>
          {theme.dark ? 'Light mode' : 'Dark mode'}
        </button>

        <!-- Signed-in user + sign out -->
        {#if userEmail}
          <div class="mt-1 rounded-lg px-3 py-2" style="border-top: 1px solid var(--sempa-border);">
            <p class="truncate text-[11px]" style="color: var(--sempa-text-dim);" title={userEmail}>{userEmail}</p>
            <button onclick={async () => { await api.auth.logout(); goto('/login'); }}
                    class="mt-0.5 text-[11px] transition-colors"
                    style="color: var(--sempa-text-dim);"
                    onmouseenter={(e) => (e.currentTarget as HTMLElement).style.color = 'var(--sempa-accent)'}
                    onmouseleave={(e) => (e.currentTarget as HTMLElement).style.color = 'var(--sempa-text-dim)'}>
              Sign out
            </button>
          </div>
        {/if}
      </div>
    </nav>
  </aside>

  <!-- ── Main content ───────────────────────────────────────────────────── -->
  <div class="flex-1 overflow-auto" style="background: var(--sempa-bg-main);">
    {@render children()}
  </div>
</div>

{#if pomodoro.taskId}
  <PomodoroTimer />
{/if}

<!-- Keyboard shortcuts help modal -->
{#if shortcutsOpen}
  <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
  <div class="fixed inset-0 z-50 bg-black/30 backdrop-blur-sm dark:bg-black/50"
       onclick={() => shortcutsOpen = false}></div>
  <div class="fixed inset-0 z-50 flex items-center justify-center pointer-events-none">
    <div class="w-80 rounded-2xl border border-gray-200 bg-white p-6 shadow-2xl pointer-events-auto
                dark:border-gray-700 dark:bg-gray-900">
      <div class="mb-4 flex items-center justify-between">
        <h3 class="text-sm font-semibold text-gray-800 dark:text-gray-100">Keyboard shortcuts</h3>
        <button onclick={() => shortcutsOpen = false}
                class="text-gray-400 hover:text-gray-600 dark:text-gray-600 dark:hover:text-gray-400">
          <X size={16} />
        </button>
      </div>
      <div class="flex flex-col gap-3">
        {#each SHORTCUT_HELP as s}
          <div class="flex items-center justify-between">
            <span class="text-sm text-gray-600 dark:text-gray-400">{s.desc}</span>
            <kbd class="rounded bg-gray-100 px-2 py-0.5 font-mono text-xs text-gray-700
                        dark:bg-gray-800 dark:text-gray-300">{s.key}</kbd>
          </div>
        {/each}
      </div>
    </div>
  </div>
{/if}
{/if}
