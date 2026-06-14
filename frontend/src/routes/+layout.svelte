<script lang="ts">
  import '../app.css';
  import { page } from '$app/stores';
  import { onMount } from 'svelte';
  import { goto, afterNavigate } from '$app/navigation';
  import { today, weekStart, offsetDate } from '$lib/utils';
  import { pomodoro } from '$lib/stores/pomodoro.svelte';
  import { theme } from '$lib/stores/theme.svelte';
  import { prefs } from '$lib/stores/prefs.svelte';
  import { tagStore } from '$lib/stores/tags.svelte';
  import { mobile } from '$lib/stores/mobile.svelte';
  import { viewport } from '$lib/stores/viewport.svelte';
  import { hapticTick } from '$lib/haptics';
  import { initPushNotifications } from '$lib/push';
  import { registerServiceWorker, enableWebPush, listenForPushNavigation, isWebPushSupported, notificationPermission } from '$lib/webpush';
  import { routines } from '$lib/stores/routines.svelte';
  import { initLocalReminders, syncLocalReminders } from '$lib/localReminders';
  import RoutineBanner from '$lib/components/RoutineBanner.svelte';
  import ReminderBanner from '$lib/components/ReminderBanner.svelte';
  import { reminderAlerts } from '$lib/stores/reminderAlerts.svelte';
  import { initDesktopReminderPopup, syncDesktopPopup } from '$lib/desktopReminderPopup';
  import { ensureDesktopNotifyPermission } from '$lib/desktopNotify';
  import { SplashScreen } from '@capacitor/splash-screen';
  import { Capacitor } from '@capacitor/core';
  import { api, getServerUrl, getTauriToken, clearTauriToken, clearNativeToken, resetApiResolver } from '$lib/api';
  import { isTauri, hasLocalDb, onSyncTrigger } from '$lib/tauri/bridge';
  import { startSync, sync as runSync, syncStore } from '$lib/sync.svelte';
  import PomodoroTimer from '$lib/components/PomodoroTimer.svelte';
  import BottomSheet from '$lib/components/BottomSheet.svelte';
  import TitleBar from '$lib/components/TitleBar.svelte';
  import SyncIndicator from '$lib/components/SyncIndicator.svelte';
  import UpdateToast from '$lib/components/UpdateToast.svelte';
  import { updates } from '$lib/stores/updates.svelte';
  import { realtime } from '$lib/stores/realtime.svelte';
  import type { Snippet } from 'svelte';

  // Lucide icons
  import {
    Sun, CalendarDays, ClipboardCheck, Mail, Moon, SlidersHorizontal,
    ChevronLeft, ChevronRight, Plus, RefreshCw, X, Check,
    Target, Timer, Palette, Menu, Layers, BookOpen, Search, Bell, LayoutGrid,
    SquareKanban, CalendarClock, ArrowUpCircle,
  } from 'lucide-svelte';
  import { createWidgetWindow } from '$lib/tauri/bridge';

  let { children }: { children: Snippet } = $props();

  const todayDate = today();
  const thisWeek  = weekStart(todayDate);

  let isLoginPage      = $derived(($page.url.pathname as string) === '/login');
  let isSetupPage      = $derived(($page.url.pathname as string) === '/setup');
  // The reminder popup is a separate, chromeless Tauri window (top-right floating
  // card). It must NOT render the app shell or run the heavy auth/sync onMount.
  let isReminderPopup  = $derived(($page.url.pathname as string) === '/reminder-popup');
  // The floating widget and sticky-note windows are likewise standalone, chromeless
  // Tauri windows — they own their entire UI and must never render the app shell
  // (sidebar / mobile bottom nav), which was bleeding into the narrow widget.
  let isStandaloneWindow = $derived(
    isReminderPopup ||
    ($page.url.pathname as string) === '/widget' ||
    ($page.url.pathname as string) === '/sticky'
  );
  let shortcutsOpen      = $state(false);
  let userEmail          = $state<string | undefined>(undefined);

  // The account line must show a real email, never a device/platform fallback
  // ("desktop"/"mobile" were placeholders used when offline). Prefer the live
  // userEmail when it looks like an email, else the email persisted at login.
  const accountEmail = $derived.by(() => {
    if (userEmail && userEmail.includes('@')) return userEmail;
    const stored = typeof localStorage !== 'undefined' ? localStorage.getItem('sempa_account_email') : null;
    return stored ?? undefined;
  });
  const accountPicture = $derived(
    typeof localStorage !== 'undefined' ? (localStorage.getItem('sempa_account_picture') ?? undefined) : undefined,
  );
  let moreSheetOpen      = $state(false);
  let showIntroAnimation = $state(false);
  let introFadingOut     = $state(false);

  // Mobile: is this a task-list page where we show the FAB?
  let isTaskListPage = $derived(
    $page.url.pathname.startsWith('/day/') || $page.url.pathname.startsWith('/week/')
  );

  const SHORTCUT_HELP = [
    { key: 'n',   desc: 'New task (day view)' },
    { key: 'e',   desc: 'Edit hovered task (day view)' },
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
      case 't': e.preventDefault(); goto('/home'); break;
      case 'j': e.preventDefault(); goto(`/day/${offsetDate(curWs, -7)}`); break;
      case 'k': e.preventDefault(); goto(`/day/${offsetDate(curWs, 7)}`); break;
      case '?': e.preventDefault(); shortcutsOpen = true; break;
    }
  }

  onMount(async () => {
    theme.init();
    // Standalone side windows (reminder popup, widget, sticky) need the theme
    // tokens but must NOT start sync/realtime/auth — those belong to the main
    // window, and the widget queries the local DB directly.
    if (isStandaloneWindow) return;

    prefs.init();
    mobile.init();
    viewport.init();

    // Background check for a newer release (throttled to ~6h; honours the user's
    // "Automatic checks" preference). Surfaces the rail indicator + update toast.
    updates.maybeAutoCheck();

    // Desktop floating reminder card (Tauri only; self-guards). Binds the popup
    // window's action listeners once, in the main window.
    void initDesktopReminderPopup((url) => goto(url));
    // Pre-warm native OS notification permission on desktop so the first fired
    // reminder shows its toast immediately instead of racing a permission prompt.
    void ensureDesktopNotifyPermission();

    // Tray "Sync Now" → run a sync cycle. Listener lives for the app's lifetime.
    if (isTauri()) {
      void onSyncTrigger(() => { void runSync(); });
    }

    // Near-real-time reconnect sync (local-first platforms only). The browser
    // 'online' event doesn't fire when a Tailscale tailnet reconnects on
    // already-connected wifi, so we trigger sync on signals that actually mean
    // "the server might be reachable again":
    //   • SSE recovers after a drop (most reliable — proves the server is up)
    //   • the app is brought back to the foreground / window regains focus
    if (hasLocalDb()) {
      realtime.onReconnect(() => { void runSync(); });
      const wake = () => { if (!document.hidden) void runSync(); };
      document.addEventListener('visibilitychange', wake);
      window.addEventListener('focus', wake);
    }

    if (!isLoginPage && !isSetupPage) {
      tagStore.load();

      // In Tauri (desktop), require server URL and token before proceeding.
      if (isTauri()) {
        if (!getServerUrl()) {
          goto('/login?redirect=' + encodeURIComponent($page.url.pathname), { replaceState: true });
          return;
        }
        if (!getTauriToken()) {
          goto('/login?redirect=' + encodeURIComponent($page.url.pathname), { replaceState: true });
          return;
        }
        // Server URL and token present. Verify the token *if* the server is
        // reachable, but never bounce to login just because we're offline — the
        // app is local-first, so a stored token means "stay signed in" and we
        // run on local data until the tailnet comes back. Only a definitive
        // "not authenticated" response (server reachable, token rejected)
        // clears the session.
        try {
          const me = await api.auth.me();
          if (!me.authenticated) {
            clearTauriToken();
            goto('/login?redirect=' + encodeURIComponent($page.url.pathname), { replaceState: true });
            return;
          }
          userEmail = me.email ?? 'desktop';
          if (me.email) localStorage.setItem('sempa_account_email', me.email);
        } catch {
          // Network error → offline. Proceed on local data; sync reconciles later.
          // accountEmail falls back to the email persisted at the last login.
          userEmail = 'desktop';
        }
        startSync();
        realtime.connect();
        showIntroAnimation = true;
        setTimeout(() => { introFadingOut = true; }, 1600);
        setTimeout(() => { showIntroAnimation = false; introFadingOut = false; }, 1800);
        return;
      }

      try {
        const me = await api.auth.me();
        if (!me.authenticated) {
          goto('/login?redirect=' + encodeURIComponent($page.url.pathname), { replaceState: true });
          return;
        }
        userEmail = me.email;
        if (me.email) localStorage.setItem('sempa_account_email', me.email);
        // Register for push notifications after auth confirmed
        initPushNotifications();
        // Web Push (PWA / browser): register the service worker, bridge its
        // deep-link messages to the router, and refresh the subscription if the
        // user already granted notification permission on a previous visit.
        listenForPushNavigation((url) => goto(url));
        void registerServiceWorker();
        if (isWebPushSupported() && notificationPermission() === 'granted') {
          void enableWebPush();
        }
        // Redirect to first-run wizard if setup hasn't been completed
        const setup = await api.setup.status();
        if (!setup.done) {
          goto('/setup', { replaceState: true });
        }
        if (hasLocalDb()) startSync();
        realtime.connect();
      } catch {
        // On Android (local-first) an unreachable server means offline, not
        // signed-out: stay in the app on local data and let sync reconcile.
        // On plain web there's no local store, so fall back to the login page.
        if (hasLocalDb()) {
          userEmail = 'mobile';
          startSync();
        } else {
          goto('/login?redirect=' + encodeURIComponent($page.url.pathname), { replaceState: true });
          realtime.disconnect();
        }
      } finally {
        if (Capacitor.isNativePlatform()) {
          await SplashScreen.hide({ fadeOutDuration: 400 });
        }
        showIntroAnimation = true;
        setTimeout(() => { introFadingOut = true; }, 1600);
        setTimeout(() => { showIntroAnimation = false; introFadingOut = false; }, 1800);
      }
    }
  });

  // Tags are loaded in onMount, but the shared layout does NOT re-mount after a
  // client-side login navigation (login uses goto), so onMount's load never
  // re-runs and tag colours stay grey until a manual reload. afterNavigate
  // fires on every client navigation — including the post-login one and the
  // initial load — so reload tags whenever we land on an authenticated page.
  afterNavigate(({ to }) => {
    const path = to?.url.pathname;
    if (path && path !== '/login' && path !== '/setup') {
      tagStore.load();
      // Same remount problem bites sync: on a FRESH login the onMount sync gate
      // already returned early (no token yet) and never runs again, so the
      // first session after sign-in never pulled and the app stayed empty until
      // a restart. startSync()/realtime.connect() self-guard against double
      // starts, so calling them here on every authenticated landing is safe and
      // guarantees sync runs the moment we're signed in on a local-first client.
      if (hasLocalDb()) {
        startSync();
        realtime.connect();
      }
      // Arm the in-app routine scheduler (weekly planning / daily shutdown
      // prompts). Idempotent: re-calling on every authenticated landing is safe.
      routines.init((url) => goto(url));
      // Schedule on-device OS alarms for upcoming reminders (Android only —
      // fires even with no server/internet). Coalesced + diff-based.
      initLocalReminders((url) => goto(url));
    }
  });

  // Keep the desktop floating reminder card in sync with fired reminders. Runs
  // in the main window only (the popup window short-circuits onMount), and
  // self-guards to Tauri inside syncDesktopPopup.
  $effect(() => {
    if (isStandaloneWindow) return;
    void syncDesktopPopup(reminderAlerts.alerts);
  });

  // Re-load tags from server when a tag:change SSE event arrives
  $effect(() => {
    const ev = realtime.lastEvent;
    if (!ev) return;
    if (ev.type === 'tag:change') tagStore.reload();
    // On local-first clients a server-side change (from another device) should
    // pull into the local DB so the UI reflects it. runSync is coalesced/cheap.
    if (hasLocalDb()) void runSync();
  });

  // When a local-first pull writes rows (revision bumps), the local DB has new
  // data but the already-mounted pages still show their initial snapshot. Route
  // the change through the realtime channel the pages already watch so they
  // re-read — this is what makes a freshly-launched app fill in once the first
  // sync lands, instead of staying empty until a manual reload.
  let lastRevision = 0;
  $effect(() => {
    const rev = syncStore.revision;
    if (rev === lastRevision) return;
    lastRevision = rev;
    tagStore.reload();
    realtime.emitLocal('task:change');
    // New/changed tasks may carry reminders — reschedule on-device alarms.
    void syncLocalReminders();
  });

  function isActive(prefix: string): boolean {
    return $page.url.pathname.startsWith(prefix);
  }

  // Desktop nav rail grouping. Items keep their routes/icons — only the order
  // and grouping change, driven by the prefs store (Settings → Appearance).
  // Recomputed via $derived so thisWeek/todayDate stay current across day rollover.
  type NavEntry = [href: string, label: string, icon: any];
  type NavGroup = { label: string | null; items: NavEntry[] };
  const navGroups = $derived.by<NavGroup[]>(() => {
    const schemes: Record<string, NavGroup[]> = {
      spaces: [
        { label: null,      items: [['/home', 'Today', CalendarDays], [`/week/${thisWeek}`, 'This Week', CalendarDays]] },
        { label: 'Rituals', items: [[`/plan/${todayDate}`, 'Plan Day', ClipboardCheck], [`/shutdown/${todayDate}`, 'Shutdown', Moon]] },
        { label: 'Inbox',   items: [['/email', 'Email', Mail], ['/reminders', 'Reminders', Bell]] },
        { label: 'Library', items: [['/backlog', 'Backlog', Layers], ['/journal', 'Journal', BookOpen]] },
      ],
      rhythm: [
        { label: 'Plan',   items: [[`/plan/${todayDate}`, 'Plan Day', ClipboardCheck], [`/week/${thisWeek}`, 'This Week', CalendarDays]] },
        { label: 'Focus',  items: [['/home', 'Today', CalendarDays], ['/backlog', 'Backlog', Layers]] },
        { label: 'Inbox',  items: [['/email', 'Email', Mail], ['/reminders', 'Reminders', Bell]] },
        { label: 'Review', items: [[`/shutdown/${todayDate}`, 'Shutdown', Moon], ['/journal', 'Journal', BookOpen]] },
      ],
      flat: [
        { label: null, items: [
          ['/home', 'Today', CalendarDays], ['/search', 'Search', Search], [`/week/${thisWeek}`, 'This Week', CalendarDays],
          [`/plan/${todayDate}`, 'Plan Day', ClipboardCheck], ['/email', 'Email', Mail], ['/backlog', 'Backlog', Layers],
          ['/reminders', 'Reminders', Bell], [`/shutdown/${todayDate}`, 'Shutdown', Moon], ['/journal', 'Journal', BookOpen],
        ] },
      ],
    };
    return schemes[prefs.navGrouping] ?? schemes.spaces;
  });

  // Sign out. Local-first: clear the stored token and leave for /login
  // IMMEDIATELY, then tell the server best-effort. The old handler awaited the
  // logout HTTP POST first, so on Android (or any time the server was slow/
  // unreachable) the await never resolved and the button appeared dead — and
  // the mobile variant never cleared the native token at all.
  async function signOut() {
    moreSheetOpen = false;
    // Tell the server to invalidate the session, best-effort, while the token
    // is still present — but DON'T await it (a hung/offline request must not
    // block the local sign-out).
    void api.auth.logout().catch(() => {});
    clearTauriToken();
    clearNativeToken();
    localStorage.removeItem('sempa_account_email');
    localStorage.removeItem('sempa_account_picture');
    userEmail = undefined;
    resetApiResolver();
    realtime.disconnect();
    await goto('/login', { replaceState: true });
  }

  // Bottom tab nav items
  const tabs = $derived([
    { href: '/home', label: 'Today', prefix: '/home', icon: 'today' },
    { href: `/week/${thisWeek}`, label: 'Week',  prefix: '/week/', icon: 'week' },
    { href: '/journal',          label: 'Journal', prefix: '/journal', icon: 'journal' },
    { href: '#more',             label: 'More',  prefix: '__more', icon: 'more' },
  ]);
</script>

<svelte:window onkeydown={handleKeydown} />

{#if isLoginPage || isSetupPage || isStandaloneWindow}
  {@render children()}
{:else}
<div class="flex flex-col h-screen overflow-hidden" style="background: var(--sempa-bg-main);">
  <!-- Custom titlebar (Tauri only — hidden on web/mobile) -->
  <TitleBar />
  <div class="flex flex-1 overflow-hidden" style="min-height: 0;">

  <!-- ── Sidebar (hidden on mobile) ───────────────────────────────────── -->
  {#if !mobile.value}
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

      {#snippet navLabel(text: string)}
        <div class="px-3 pb-1.5 pt-3.5 font-mono text-[10px] font-semibold uppercase tracking-[0.13em]"
             style="color: var(--sempa-text-dim);">{text}</div>
      {/snippet}

      <!-- Pinned Search pill (grouped schemes only — in Flat, Search is a row). -->
      {#if prefs.navGrouping !== 'flat'}
        <button onclick={() => goto('/search')}
                class="mb-3 flex w-full items-center gap-2 rounded-lg px-3 py-2 text-[13px] transition-colors"
                style="background: var(--card-bg); border: 1px solid var(--sempa-border); color: var(--sempa-text-dim);"
                onmouseenter={(e) => (e.currentTarget as HTMLElement).style.borderColor = 'var(--sempa-text-dim)'}
                onmouseleave={(e) => (e.currentTarget as HTMLElement).style.borderColor = 'var(--sempa-border)'}>
          <Search size={15} strokeWidth={1.75} />
          <span>Search</span>
          <kbd class="ml-auto rounded border px-1.5 font-mono text-[10.5px]"
               style="border-color: var(--sempa-border);">⌘K</kbd>
        </button>
      {/if}

      <!-- Sectioned nav (grouping + section style from Settings → Appearance). -->
      {#each navGroups as group, gi}
        <div class="flex flex-col gap-0.5" class:mt-3.5={gi > 0 && prefs.navSections === 'labels' && !!group.label}>
          {#if prefs.navSections === 'dividers' && gi > 0}
            <div class="mx-3 mb-2 mt-1.5 h-px" style="background: var(--sempa-border);"></div>
          {/if}
          {#if prefs.navSections === 'labels' && group.label}
            {@render navLabel(group.label)}
          {/if}
          {#each group.items as [href, label, Icon]}
            {@render navItem(href, label, Icon)}
          {/each}
        </div>
      {/each}

      <!-- Pomodoro in-progress -->
      {#if pomodoro.taskId}
        <div class="mt-2 rounded-xl border px-3 py-2.5"
             style="border-color: var(--sempa-accent-bg); background: var(--sempa-accent-bg); color: var(--sempa-accent);">
          <p class="truncate text-[10.5px] font-semibold uppercase tracking-wider opacity-70">
            {pomodoro.phaseLabel}
          </p>
          <p class="font-mono text-xl font-bold">{pomodoro.display}</p>
        </div>
      {/if}

      <!-- Rail footer: utility icons spread across the row, the sync status, and
           a full-width account chip (avatar + email + Sign out). Spreading the
           icons (justify-between) and giving sync + account their own rows keeps
           everything legible — no cramped clump, no orphaned avatar. -->
      <div class="mt-auto flex flex-col gap-3.5 pt-4" style="border-top: 1px solid var(--sempa-border);">
        <!-- Utility icon row -->
        <div class="flex items-center justify-between">
          {#if updates.available}
            <button onclick={() => goto('/settings/accounts')} title="Update available — open About"
                    aria-label="Update available"
                    class="flex h-[34px] w-[34px] items-center justify-center rounded-[9px] transition-colors"
                    style="background: var(--sempa-accent-bg); color: var(--sempa-accent);">
              <ArrowUpCircle size={18} strokeWidth={1.9} />
            </button>
          {/if}
          <button onclick={() => goto('/settings/accounts')} title="Settings" aria-label="Settings"
                  class="flex h-[34px] w-[34px] items-center justify-center rounded-[9px] transition-colors"
                  style={isActive('/settings') ? 'background: var(--sempa-accent-bg); color: var(--sempa-accent);' : 'color: var(--sempa-text-dim);'}
                  onmouseenter={(e) => { if (!isActive('/settings')) { const t = e.currentTarget as HTMLElement; t.style.background = 'var(--sempa-accent-bg)'; t.style.color = 'var(--sempa-text-soft)'; } }}
                  onmouseleave={(e) => { if (!isActive('/settings')) { const t = e.currentTarget as HTMLElement; t.style.background = ''; t.style.color = 'var(--sempa-text-dim)'; } }}>
            <SlidersHorizontal size={18} strokeWidth={1.75} />
          </button>

          <button onclick={theme.toggle} title={theme.dark ? 'Light mode' : 'Dark mode'}
                  aria-label={theme.dark ? 'Switch to light mode' : 'Switch to dark mode'}
                  class="flex h-[34px] w-[34px] items-center justify-center rounded-[9px] transition-colors"
                  style="color: var(--sempa-text-dim);"
                  onmouseenter={(e) => { const t = e.currentTarget as HTMLElement; t.style.background = 'var(--sempa-accent-bg)'; t.style.color = 'var(--sempa-text-soft)'; }}
                  onmouseleave={(e) => { const t = e.currentTarget as HTMLElement; t.style.background = ''; t.style.color = 'var(--sempa-text-dim)'; }}>
            {#if theme.dark}<Sun size={18} strokeWidth={1.75} />{:else}<Moon size={18} strokeWidth={1.75} />{/if}
          </button>

          {#if isTauri()}
            <button onclick={() => createWidgetWindow()} title="Open Widget" aria-label="Open the floating desktop widget"
                    class="flex h-[34px] w-[34px] items-center justify-center rounded-[9px] transition-colors"
                    style="color: var(--sempa-text-dim);"
                    onmouseenter={(e) => { const t = e.currentTarget as HTMLElement; t.style.background = 'var(--sempa-accent-bg)'; t.style.color = 'var(--sempa-text-soft)'; }}
                    onmouseleave={(e) => { const t = e.currentTarget as HTMLElement; t.style.background = ''; t.style.color = 'var(--sempa-text-dim)'; }}>
              <LayoutGrid size={18} strokeWidth={1.75} />
            </button>
          {/if}
        </div>

        <!-- Sync status (renders nothing on plain web). -->
        <SyncIndicator />

        <!-- Account chip — avatar + identity; the "Sign out" line is the affordance. -->
        <button onclick={signOut} title={accountEmail ? `${accountEmail} — sign out` : 'Sign out'}
                aria-label="Sign out"
                class="flex w-full items-center gap-2.5 rounded-[10px] px-2 py-1.5 text-left transition-colors"
                onmouseenter={(e) => (e.currentTarget as HTMLElement).style.background = 'var(--sempa-accent-bg)'}
                onmouseleave={(e) => (e.currentTarget as HTMLElement).style.background = ''}>
          {#if accountPicture}
            <img src={accountPicture} alt="" referrerpolicy="no-referrer"
                 class="h-7 w-7 shrink-0 rounded-full object-cover" style="border: 1px solid var(--sempa-border);" />
          {:else}
            <span class="flex h-7 w-7 shrink-0 items-center justify-center rounded-full"
                  style="background: var(--sempa-accent-bg); border: 1px solid var(--sempa-border);
                         color: var(--sempa-accent); font-size: 11.5px; font-weight: 700;">
              {(accountEmail ?? '?').charAt(0).toUpperCase()}
            </span>
          {/if}
          <span class="min-w-0 flex-1">
            {#if accountEmail}
              <span class="block truncate text-[12px]" style="color: var(--sempa-text-soft);">{accountEmail}</span>
            {/if}
            <span class="block text-[11px]" style="color: var(--sempa-text-dim);">Sign out</span>
          </span>
        </button>
      </div>
    </nav>
  </aside>
  {/if}

  <!-- ── Main content ───────────────────────────────────────────────────── -->
  <div class="flex-1 overflow-auto" style="background: var(--sempa-bg-main);
       {mobile.value ? 'padding-bottom: 88px;' : ''}">
    <ReminderBanner />
    <RoutineBanner />
    {#key $page.url.pathname}
      <div class="animate-page-in">{@render children()}</div>
    {/key}
  </div>
  </div><!-- end inner flex row -->
</div>

<!-- ── Mobile bottom tab bar ────────────────────────────────────────────── -->
{#if mobile.value}
  <nav style="position: fixed; bottom: 0; left: 0; right: 0; height: 72px; z-index: 40;
              background: var(--sempa-bg-nav); border-top: 1px solid var(--sempa-border);
              display: flex; align-items: center; padding-bottom: env(safe-area-inset-bottom);">
    {#each tabs as tab}
      {@const active = tab.prefix === '__more' ? moreSheetOpen : isActive(tab.prefix)}
      <button
        onclick={() => {
          if (tab.prefix === '__more') { moreSheetOpen = true; }
          else { goto(tab.href); moreSheetOpen = false; }
        }}
        style="display: flex; flex-direction: column; align-items: center; gap: 3px; flex: 1;
               background: none; border: none; cursor: pointer;
               color: {active ? 'var(--sempa-accent)' : 'var(--sempa-text-dim)'};">
        {#if tab.icon === 'today'}
          <CalendarDays size={22} strokeWidth={active ? 2.25 : 1.75} />
        {:else if tab.icon === 'week'}
          <CalendarDays size={22} strokeWidth={active ? 2.25 : 1.75} />
        {:else if tab.icon === 'inbox'}
          <Mail size={22} strokeWidth={active ? 2.25 : 1.75} />
        {:else if tab.icon === 'journal'}
          <BookOpen size={22} strokeWidth={active ? 2.25 : 1.75} />
        {:else}
          <Menu size={22} strokeWidth={active ? 2.25 : 1.75} />
        {/if}
        <span style="font-family: 'Plus Jakarta Sans', sans-serif; font-size: 11px;
                     font-weight: {active ? '600' : '400'};">{tab.label}</span>
      </button>
    {/each}
  </nav>

  <!-- FAB for task creation on task-list pages -->
  {#if isTaskListPage}
    <button
      onclick={() => { hapticTick(); goto(`/day/${todayDate}?new=1`); }}
      aria-label="New task"
      style="position: fixed; bottom: calc(72px + env(safe-area-inset-bottom, 0px) + 12px); right: 20px; width: 52px; height: 52px;
             border-radius: 16px; background: var(--sempa-btn-bg); color: var(--sempa-btn-fg);
             display: flex; align-items: center; justify-content: center; z-index: 30;
             box-shadow: 0 4px 16px rgba(0,0,0,0.25); border: none; cursor: pointer;">
      <Plus size={22} strokeWidth={2.5} />
    </button>
  {/if}

  <!-- "More" bottom sheet — sectioned: quick row, grouped destinations, account -->
  <BottomSheet open={moreSheetOpen} onClose={() => moreSheetOpen = false}>
    <div style="padding: 8px 16px 24px;">

      {#snippet sheetTile(href: string, label: string, Icon: any)}
        {@const active = isActive(href)}
        <a {href} onclick={() => moreSheetOpen = false}
           class="flex items-center gap-2.5 rounded-xl px-3 transition-colors"
           style="min-height: 48px; background: var(--sempa-bg-nav);
                  {active ? 'color: var(--sempa-accent); font-weight: 600;' : 'color: var(--sempa-text-soft);'}">
          <Icon size={18} strokeWidth={active ? 2.25 : 1.75} />
          <span style="font-size: 14px;">{label}</span>
        </a>
      {/snippet}

      <!-- Quick row -->
      <div class="flex gap-2">
        <button onclick={() => { goto('/settings/accounts'); moreSheetOpen = false; }}
                aria-label="Settings"
                class="flex flex-1 flex-col items-center justify-center gap-1.5 rounded-xl py-3 transition-opacity active:opacity-80"
                style="background: var(--sempa-accent-bg); color: var(--sempa-accent);">
          <SlidersHorizontal size={20} strokeWidth={1.9} />
          <span style="font-size: 12.5px; font-weight: 600;">Settings</span>
        </button>
        <button onclick={() => theme.toggle()}
                aria-label={theme.dark ? 'Switch to light mode' : 'Switch to dark mode'}
                class="flex flex-1 flex-col items-center justify-center gap-1.5 rounded-xl py-3 transition-opacity active:opacity-80"
                style="background: var(--sempa-accent-bg); color: var(--sempa-accent);">
          {#if theme.dark}<Sun size={20} strokeWidth={1.9} />{:else}<Moon size={20} strokeWidth={1.9} />{/if}
          <span style="font-size: 12.5px; font-weight: 600;">{theme.dark ? 'Light mode' : 'Dark mode'}</span>
        </button>
        {#if isTauri()}
          <button onclick={() => createWidgetWindow()} aria-label="Open widget"
                  class="flex flex-1 flex-col items-center justify-center gap-1.5 rounded-xl py-3 transition-opacity active:opacity-80"
                  style="background: var(--sempa-accent-bg); color: var(--sempa-accent);">
            <LayoutGrid size={20} strokeWidth={1.9} />
            <span style="font-size: 12.5px; font-weight: 600;">Widget</span>
          </button>
        {/if}
      </div>

      <!-- Plan -->
      <p class="mb-2 mt-4 px-1 text-[10.5px] font-semibold uppercase tracking-wider" style="color: var(--sempa-text-dim);">Plan</p>
      <div class="grid grid-cols-2 gap-2">
        {@render sheetTile(`/plan/${todayDate}`, 'Plan Day', ClipboardCheck)}
        {@render sheetTile('/schedule', 'Schedule', CalendarClock)}
        {@render sheetTile('/backlog', 'Backlog', Layers)}
        {@render sheetTile('/search', 'Search', Search)}
      </div>

      <!-- Inbox -->
      <p class="mb-2 mt-4 px-1 text-[10.5px] font-semibold uppercase tracking-wider" style="color: var(--sempa-text-dim);">Inbox</p>
      <div class="grid grid-cols-2 gap-2">
        {@render sheetTile('/email', 'Email', Mail)}
        {@render sheetTile('/reminders', 'Reminders', Bell)}
        {@render sheetTile('/jira', 'Jira', SquareKanban)}
        {@render sheetTile(`/shutdown/${todayDate}`, 'Shutdown', Moon)}
      </div>

      <!-- Pomodoro (if running) -->
      {#if pomodoro.taskId}
        <div class="mt-4 rounded-xl px-4 py-3"
             style="background: var(--sempa-accent-bg); color: var(--sempa-accent);">
          <p class="text-[10.5px] font-semibold uppercase tracking-wider opacity-70">{pomodoro.phaseLabel}</p>
          <p class="font-mono text-xl font-bold">{pomodoro.display}</p>
        </div>
      {/if}

      <!-- Sync status (local-first platforms only) -->
      <div class="mt-3 px-1"><SyncIndicator /></div>

      <!-- Account -->
      <div class="mt-3 flex items-center gap-3 rounded-xl px-3 py-3" style="border: 1px solid var(--sempa-border);">
        {#if accountPicture}
          <img src={accountPicture} alt="" referrerpolicy="no-referrer"
               class="h-8 w-8 shrink-0 rounded-full object-cover" style="border: 1px solid var(--sempa-border);" />
        {:else}
          <span class="flex h-8 w-8 shrink-0 items-center justify-center rounded-full"
                style="background: var(--sempa-accent-bg); border: 1px solid var(--sempa-border);
                       color: var(--sempa-accent); font-size: 12px; font-weight: 700;">
            {(accountEmail ?? '?').charAt(0).toUpperCase()}
          </span>
        {/if}
        <div class="min-w-0 flex-1">
          {#if accountEmail}
            <p class="truncate text-[13px]" style="color: var(--sempa-text-soft);" title={accountEmail}>{accountEmail}</p>
          {/if}
        </div>
        <button onclick={signOut}
                class="shrink-0 rounded-lg px-3 py-1.5 text-[13px] font-medium transition-opacity active:opacity-80"
                style="color: var(--sempa-accent); background: var(--sempa-accent-bg);">
          Sign out
        </button>
      </div>
    </div>
  </BottomSheet>
{/if}

{#if pomodoro.taskId}
  <PomodoroTimer />
{/if}

<!-- ── In-app update toast (main window only) ───────────────────────────── -->
{#if !isStandaloneWindow}
  <UpdateToast />
{/if}

<!-- ── Intro animation overlay ──────────────────────────────────────────── -->
{#if showIntroAnimation}
  <div class="intro-overlay" style="opacity:{introFadingOut ? '0' : '1'};">
    <svg width="80" height="80" viewBox="0 0 100 100" fill="none" aria-hidden="true">
      <path class="arc" d="M22,40 a28,28 0 0 0 56,0"
            stroke="var(--sempa-accent)" stroke-width="9"
            stroke-linecap="round" stroke-linejoin="round"
            stroke-dasharray="88" stroke-dashoffset="88"/>
      <circle class="dot" cx="50" cy="35" r="7.5" fill="var(--sempa-accent)"/>
    </svg>
    <span class="wordmark">sempa</span>
  </div>
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

<style>
  .intro-overlay {
    position: fixed;
    inset: 0;
    z-index: 100;
    background: var(--sempa-bg-main);
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 16px;
    transition: opacity 200ms ease-out;
  }

  @media (prefers-reduced-motion: no-preference) {
    .intro-overlay :global(.arc) {
      animation: arc-draw 700ms ease-out 0ms forwards;
    }
    .intro-overlay :global(.dot) {
      transform-origin: 50px 35px;
      transform: scale(0);
      animation: dot-pop 350ms cubic-bezier(0.34, 1.56, 0.64, 1) 400ms forwards;
    }
    .intro-overlay :global(.wordmark) {
      opacity: 0;
      transform: translateY(8px);
      animation: wordmark-in 400ms ease-out 600ms forwards;
    }
  }

  .wordmark {
    font-family: 'Plus Jakarta Sans', sans-serif;
    font-weight: 500;
    font-size: 24px;
    letter-spacing: -0.02em;
    color: var(--sempa-text);
  }

  @keyframes arc-draw {
    to { stroke-dashoffset: 0; }
  }
  @keyframes dot-pop {
    to { transform: scale(1); }
  }
  @keyframes wordmark-in {
    to { opacity: 1; transform: translateY(0); }
  }
</style>
