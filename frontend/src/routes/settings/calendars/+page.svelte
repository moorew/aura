<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api';
  import type { ICalSubscription } from '$lib/types';
  import { calendars, calFg, BRAND_CAL_LABEL } from '$lib/stores/calendars.svelte';

  // Connected calendar accounts
  let google   = $state<{ connected: boolean; email?: string; last_synced_at?: string }>({ connected: false });
  let fastmail = $state<{ connected: boolean; enabled?: boolean; last_synced_at?: string | null }>({ connected: false });
  let fmEmail  = $state<string | undefined>();

  // Subscribed calendars (ICS feeds) — the per-calendar entities the schedule
  // colours and show/hide settings key off of (matching ev.subscription_id).
  let subs    = $state<ICalSubscription[]>([]);
  let loading = $state(true);

  onMount(async () => {
    const [g, f, fm, s] = await Promise.allSettled([
      api.integrations.calendar.get(),
      api.integrations.fastmail.calendar.get(),
      api.integrations.fastmail.get(),
      api.ical.listSubscriptions(),
    ]);
    if (g.status === 'fulfilled')  google   = g.value;
    if (f.status === 'fulfilled')  fastmail = f.value;
    if (fm.status === 'fulfilled') fmEmail  = (fm.value as { email?: string }).email;
    if (s.status === 'fulfilled')  subs     = s.value;
    loading = false;
  });

  function fmtTime(s?: string | null) {
    if (!s) return 'Never';
    return new Date(s).toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' });
  }

  type Account = { id: string; name: string; email?: string; synced?: string | null };
  const accounts = $derived.by(() => {
    const out: Account[] = [];
    if (google.connected)   out.push({ id: 'google',   name: 'Google Calendar',   email: google.email,  synced: google.last_synced_at });
    if (fastmail.connected) out.push({ id: 'fastmail', name: 'Fastmail Calendar', email: fmEmail,        synced: fastmail.last_synced_at });
    return out;
  });

  // ── Add feed modal ──────────────────────────────────────────────────────────
  let showAddFeed = $state(false);
  let feedUrl     = $state('');
  let feedName    = $state('');
  let feedError   = $state('');
  let feedSaving  = $state(false);

  function openAddFeed() {
    feedUrl = ''; feedName = ''; feedError = ''; showAddFeed = true;
  }

  // Accept http/https/webcal only; webcal is just https for an ICS feed, so the
  // backend (which fetches http(s)) gets a normalised URL.
  function validFeedUrl(raw: string): boolean {
    return /^(https?|webcal):\/\//i.test(raw.trim());
  }

  async function saveFeed() {
    const raw = feedUrl.trim();
    if (!validFeedUrl(raw)) {
      feedError = 'Enter a URL starting with http, https, or webcal.';
      return;
    }
    feedSaving = true; feedError = '';
    try {
      const normalizedUrl = raw.replace(/^webcal:\/\//i, 'https://');
      const sub = await api.ical.createSubscription({
        name: feedName.trim() || new URL(normalizedUrl).hostname,
        url:  normalizedUrl,
      });
      subs = [...subs, sub];
      showAddFeed = false;
    } catch (e) {
      feedError = (e as Error).message || 'Failed to add feed.';
    } finally {
      feedSaving = false;
    }
  }
</script>

<div class="mx-auto flex h-full max-w-xl flex-col" style="padding-top: env(safe-area-inset-top, 0px);">
  <!-- Header -->
  <div class="flex items-center gap-3 px-5 py-4" style="border-bottom: 1px solid var(--sempa-border);">
    <a href="/settings/accounts"
       class="flex items-center gap-1.5 rounded-lg px-2 py-1.5 text-sm transition-colors"
       style="color: var(--sempa-accent);">
      <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
        <path stroke-linecap="round" d="M19 12H5m7-7-7 7 7 7"/>
      </svg>
      Settings
    </a>
    <h1 class="text-base font-semibold" style="color: var(--sempa-text);">Calendars</h1>
  </div>

  <div class="flex-1 overflow-y-auto px-5 py-6 pb-16">
    {#if loading}
      <p class="text-sm" style="color: var(--sempa-text-dim);">Loading calendars…</p>
    {:else}

      <!-- ── Connected accounts ─────────────────────────────────────────── -->
      <p class="mb-3" style="font-family:monospace; font-size:10.5px; font-weight:700; letter-spacing:0.12em;
         text-transform:uppercase; color:var(--sempa-text-dim)">Accounts</p>

      {#if accounts.length === 0}
        <div class="mb-7 rounded-xl border px-4 py-4 text-sm"
             style="border-color: var(--sempa-border); background: var(--sempa-bg-panel); color: var(--sempa-text-soft);">
          No calendar accounts connected yet.
          <a href="/settings/accounts" style="color: var(--sempa-accent);">Connect one →</a>
        </div>
      {:else}
        <section class="mb-7 overflow-hidden rounded-xl border" style="border-color: var(--sempa-border); background: var(--sempa-bg-panel);">
          {#each accounts as acct, i (acct.id)}
            <div class="flex items-center gap-3.5 px-4 py-3.5"
                 style={i < accounts.length - 1 ? 'border-bottom: 1px solid var(--sempa-border);' : ''}>
              <!-- Provider icon tile -->
              <div class="flex h-7 w-7 shrink-0 items-center justify-center rounded-lg" style="background: var(--sempa-accent-bg);">
                <svg class="h-4 w-4" style="color: var(--sempa-accent);" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24">
                  <rect x="3" y="4" width="18" height="18" rx="2"/><path stroke-linecap="round" d="M16 2v4M8 2v4M3 10h18"/>
                </svg>
              </div>
              <div class="min-w-0 flex-1">
                <p class="truncate font-semibold" style="font-size:13.5px; color: var(--sempa-text);">{acct.name}</p>
                {#if acct.email}
                  <p class="truncate" style="font-size:11px; color: var(--sempa-text-dim);">{acct.email}</p>
                {/if}
              </div>
              <span class="inline-flex shrink-0 items-center gap-1.5" style="font-size:11px; color: var(--sempa-text-soft);">
                <span class="rounded-full" style="width:7px; height:7px; background: var(--sempa-success);"></span>
                Connected
              </span>
            </div>
          {/each}
        </section>
      {/if}

      <!-- ── Subscribed calendars ───────────────────────────────────────── -->
      <div class="mb-3 flex items-center justify-between">
        <p style="font-family:monospace; font-size:10.5px; font-weight:700; letter-spacing:0.12em;
           text-transform:uppercase; color:var(--sempa-text-dim)">Calendars</p>
        <button onclick={openAddFeed}
                class="rounded-lg border px-2.5 py-1 text-xs font-medium transition-colors"
                style="border-color: var(--sempa-border); color: var(--sempa-text-soft);">
          + Add feed
        </button>
      </div>

      {#if subs.length === 0}
        <div class="mb-7 rounded-xl border px-4 py-4 text-sm"
             style="border-color: var(--sempa-border); background: var(--sempa-bg-panel); color: var(--sempa-text-soft);">
          No subscribed calendars yet.
          <button onclick={openAddFeed} style="color: var(--sempa-accent);">Add an iCal or webcal feed →</button>
        </div>
      {:else}
        <section class="mb-2 overflow-hidden rounded-xl border" style="border-color: var(--sempa-border); background: var(--sempa-bg-panel);">
          {#each subs as sub, i (sub.id)}
            {@const on = !calendars.isHidden(sub.id)}
            {@const colorKey = calendars.colorKey(sub.id)}
            <div class="flex items-center gap-3 px-4 py-3 transition-opacity"
                 style="opacity: {on ? 1 : 0.55}; {i < subs.length - 1 ? 'border-bottom: 1px solid var(--sempa-border);' : ''}">
              <!-- Colour swatch — tap to cycle the four brand colours -->
              <button onclick={() => calendars.cycleColor(sub.id)}
                      class="shrink-0 rounded-[5px] transition-transform active:scale-90"
                      style="width:14px; height:14px; background: {calFg(colorKey)};"
                      title="Colour: {BRAND_CAL_LABEL[colorKey]} — tap to change"
                      aria-label="Change calendar colour"></button>
              <span class="min-w-0 flex-1 truncate" style="font-size:13px; color: var(--sempa-text);">{sub.name}</span>
              <!-- On/off toggle (34×18) -->
              <button onclick={() => calendars.toggleHidden(sub.id)}
                      class="relative inline-flex shrink-0 items-center rounded-full transition-colors"
                      style="width:34px; height:18px; background: {on ? 'var(--sempa-accent)' : 'var(--sempa-border)'};"
                      aria-label={on ? 'Hide calendar' : 'Show calendar'} aria-pressed={on}>
                <span class="inline-block rounded-full bg-white shadow transition-transform"
                      style="width:14px; height:14px; transform: translateX({on ? '18px' : '2px'});"></span>
              </button>
            </div>
          {/each}
        </section>
        <p class="mb-7 px-1" style="font-size:11px; color: var(--sempa-text-dim);">
          Tap a colour swatch to cycle through Terracotta · Stone · Sage · Amber.
        </p>
      {/if}

      <!-- ── Display preferences ────────────────────────────────────────── -->
      <p class="mb-3" style="font-family:monospace; font-size:10.5px; font-weight:700; letter-spacing:0.12em;
         text-transform:uppercase; color:var(--sempa-text-dim)">Display preferences</p>

      <section class="overflow-hidden rounded-xl border" style="border-color: var(--sempa-border); background: var(--sempa-bg-panel);">
        {#each [
          { key: 'showDeclined',   label: 'Show declined events',          desc: 'Include events you’ve declined.' },
          { key: 'showAllDayWeek', label: 'Show all-day in week header',   desc: 'Surface all-day events above the week grid.' },
          { key: 'dimPastEvents',  label: 'Dim past events',               desc: 'Fade events earlier than now on today.' },
        ] as row, i (row.key)}
          {@const on = calendars.display[row.key as keyof typeof calendars.display]}
          <div class="flex items-center gap-3 px-4 py-3.5"
               style={i < 2 ? 'border-bottom: 1px solid var(--sempa-border);' : ''}>
            <div class="min-w-0 flex-1">
              <p style="font-size:13px; color: var(--sempa-text);">{row.label}</p>
              <p style="font-size:11px; color: var(--sempa-text-dim);">{row.desc}</p>
            </div>
            <button onclick={() => calendars.setDisplay(row.key as 'showDeclined' | 'showAllDayWeek' | 'dimPastEvents', !on)}
                    class="relative inline-flex shrink-0 items-center rounded-full transition-colors"
                    style="width:34px; height:18px; background: {on ? 'var(--sempa-accent)' : 'var(--sempa-border)'};"
                    aria-label={row.label} aria-pressed={on}>
              <span class="inline-block rounded-full bg-white shadow transition-transform"
                    style="width:14px; height:14px; transform: translateX({on ? '18px' : '2px'});"></span>
            </button>
          </div>
        {/each}
      </section>

    {/if}
  </div>
</div>

<!-- ── Add feed modal ─────────────────────────────────────────────────────── -->
{#if showAddFeed}
  <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
  <div class="fixed inset-0 z-[80] flex items-center justify-center bg-black/30 backdrop-blur-sm animate-fade-in"
       onclick={() => (showAddFeed = false)}>
    <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
    <div class="mx-4 w-full max-w-sm rounded-2xl p-5 shadow-2xl animate-scale-in"
         style="background: var(--sempa-bg-panel); border: 1px solid var(--sempa-border);"
         onclick={(e) => e.stopPropagation()}>
      <h3 class="mb-1 text-sm font-semibold" style="color: var(--sempa-text);">Add calendar feed</h3>
      <p class="mb-4 text-xs" style="color: var(--sempa-text-dim);">Subscribe to a read-only ICS or webcal URL.</p>

      <label class="mb-1 block text-xs font-medium" style="color: var(--sempa-text-soft);" for="feed-url">
        ICS / Webcal URL <span class="text-red-400">*</span>
      </label>
      <!-- svelte-ignore a11y_autofocus -->
      <input id="feed-url" type="url" inputmode="url" bind:value={feedUrl} autofocus
             autocomplete="off" autocapitalize="none" spellcheck="false"
             placeholder="https://example.com/calendar.ics  or  webcal://…"
             onkeydown={(e) => { if (e.key === 'Enter') saveFeed(); }}
             class="mb-3 w-full rounded-lg border px-3 py-2 text-sm outline-none"
             style="border-color: var(--sempa-border); background: var(--sempa-bg-main); color: var(--sempa-text);" />

      <label class="mb-1 block text-xs font-medium" style="color: var(--sempa-text-soft);" for="feed-name">Name (optional)</label>
      <input id="feed-name" type="text" bind:value={feedName}
             placeholder="Work calendar"
             onkeydown={(e) => { if (e.key === 'Enter') saveFeed(); }}
             class="w-full rounded-lg border px-3 py-2 text-sm outline-none"
             style="border-color: var(--sempa-border); background: var(--sempa-bg-main); color: var(--sempa-text);" />

      {#if feedError}<p class="mt-3 text-sm text-red-600 dark:text-red-400">{feedError}</p>{/if}

      <div class="mt-5 flex items-center justify-end gap-2">
        <button onclick={() => (showAddFeed = false)}
                class="rounded-lg border px-4 py-2 text-sm transition-colors"
                style="border-color: var(--sempa-border); color: var(--sempa-text-soft);">
          Cancel
        </button>
        <button onclick={saveFeed} disabled={feedSaving || !feedUrl.trim()}
                class="rounded-lg px-4 py-2 text-sm font-medium text-white transition-opacity hover:opacity-90 disabled:opacity-40"
                style="background: var(--sempa-accent);">
          {feedSaving ? 'Saving…' : 'Save'}
        </button>
      </div>
    </div>
  </div>
{/if}
