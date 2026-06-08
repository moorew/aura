<script lang="ts">
  import { onMount, tick } from 'svelte';
  import { page } from '$app/stores';
  import { api } from '$lib/api';
  import { theme, ACCENT_PRESETS, type AccentName } from '$lib/stores/theme.svelte';
  import { prefs } from '$lib/stores/prefs.svelte';
  import { mobile } from '$lib/stores/mobile.svelte';
  import { goto } from '$app/navigation';
  import type { ICalSubscription } from '$lib/types';

  type AccountStatus = { connected: boolean; email?: string; last_synced_at?: string | null; enabled?: boolean };

  let gmail    = $state<AccountStatus>({ connected: false });
  let calendar = $state<{ connected: boolean; email?: string; last_synced_at?: string }>({ connected: false });
  let fastmail = $state<AccountStatus>({ connected: false });
  let fmCal    = $state<{ connected: boolean; enabled: boolean; last_synced_at?: string | null }>({ connected: false, enabled: false });
  // CalDAV — push scheduled tasks to a calendar (reuses Fastmail credentials)
  let caldav = $state<{ connected: boolean; enabled?: boolean; calendar_href?: string; calendar_name?: string; last_synced_at?: string | null }>({ connected: false });
  let caldavCalendars = $state<{ href: string; name: string; color?: string }[]>([]);
  let caldavPickerOpen = $state(false);
  let caldavLoading = $state(false);
  let caldavError = $state('');
  let jira     = $state<{ connected: boolean; last_synced_at?: string | null }>({ connected: false });
  let taskInbox = $state<{
    connected: boolean; email?: string; inbox_address?: string;
    allowed_senders?: string[]; last_synced_at?: string;
  }>({ connected: false });

  // Fastmail connect form
  let fmEmail = $state('');
  let fmPassword = $state('');
  let fmSaving = $state(false);
  let fmError = $state('');
  let fmShowForm = $state(false);

  // Email inbox connect form
  let tiEmail = $state('');
  let tiPassword = $state('');
  let tiAddress = $state('tasks@sempa.ca');
  let tiSaving = $state(false);
  let tiError = $state('');
  let tiShowForm = $state(false);

  // Allowed senders
  let senderInput = $state('');
  let senderSaving = $state(false);

  let syncing     = $state<Record<string, boolean>>({});
  let syncResults = $state<Record<string, string>>({});

  // ICS subscriptions
  let icalSubs      = $state<ICalSubscription[]>([]);
  let icalUrl       = $state('');
  let icalName      = $state('');
  let icalColor     = $state('#6366f1');
  let icalAdding    = $state(false);
  let icalError     = $state('');
  let showIcalForm  = $state(false);
  let icalFormEl    = $state<HTMLElement | undefined>();

  // Sub-nav active section tracking
  let activeSection = $state('integrations');
  let scrollContainer = $state<HTMLElement | undefined>();

  // Mobile section navigation
  let mobileSection = $state<string | null>(null);

  const NAV_SECTIONS = [
    { id: 'integrations', label: 'Integrations' },
    { id: 'tasks', label: 'Tasks' },
    { id: 'appearance', label: 'Appearance' },
  ] as const;

  // Integrations (Gmail / Fastmail / Jira / calendar OAuth) are managed on
  // desktop/web; they're hidden in the mobile settings to keep it focused.
  const MOBILE_SECTIONS = NAV_SECTIONS.filter((s) => s.id !== 'integrations');

  // Surfaces a clear banner when the device can't reach the backend at all
  // (the common Android failure mode — wrong/missing server URL or auth). Without
  // this the section just renders every integration as "disconnected", which
  // reads as "nothing loaded".
  let serverUnreachable = $state(false);

  onMount(async () => {
    const connected = $page.url.searchParams.get('connected');
    if (connected === '1') window.history.replaceState({}, '', '/settings/accounts');

    // allSettled so one failing endpoint never rejects the batch (which would
    // leave the whole Integrations section blank). Track how many failed so we
    // can tell "server unreachable" apart from "genuinely disconnected".
    const results = await Promise.allSettled([
      api.integrations.gmail.get(),
      api.integrations.calendar.get(),
      api.integrations.fastmail.get(),
      api.integrations.fastmail.calendar.get(),
      api.integrations.taskInbox.get(),
      api.ical.listSubscriptions(),
      api.integrations.jira.get(),
      api.integrations.caldav.get(),
    ]);
    const val = <T,>(i: number, fallback: T): T =>
      results[i].status === 'fulfilled' ? (results[i] as PromiseFulfilledResult<T>).value : fallback;

    gmail     = val(0, { connected: false });
    calendar  = val(1, { connected: false });
    fastmail  = val(2, { connected: false });
    fmCal     = val(3, { connected: false, enabled: false });
    taskInbox = val(4, { connected: false });
    icalSubs  = val(5, []);
    jira      = val(6, { connected: false });
    caldav    = val(7, { connected: false });

    serverUnreachable = results.every((r) => r.status === 'rejected');

    // IntersectionObserver for sub-nav active state
    await tick();
    setupObserver();
  });

  function setupObserver() {
    if (!scrollContainer) return;
    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting) {
            activeSection = entry.target.id.replace('settings-', '');
          }
        }
      },
      { root: scrollContainer, rootMargin: '-10% 0px -80% 0px', threshold: 0 }
    );
    for (const s of NAV_SECTIONS) {
      const el = document.getElementById(`settings-${s.id}`);
      if (el) observer.observe(el);
    }
  }

  function scrollTo(id: string) {
    const el = document.getElementById(`settings-${id}`);
    el?.scrollIntoView({ behavior: 'smooth', block: 'start' });
  }

  async function addIcalSub() {
    if (!icalUrl.trim()) return;
    icalAdding = true; icalError = '';
    try {
      let parsedUrl = icalUrl.trim();
      const urlForParsing = parsedUrl.replace(/^webcal:\/\//, 'https://');
      const sub = await api.ical.createSubscription({
        name: icalName.trim() || new URL(urlForParsing).hostname,
        url:  parsedUrl,
        color: icalColor,
      });
      icalSubs = [...icalSubs, sub];
      icalUrl = ''; icalName = ''; icalColor = '#6366f1'; showIcalForm = false;
    } catch (e) { icalError = (e as Error).message; }
    finally { icalAdding = false; }
  }

  async function openIcalForm() {
    showIcalForm = true;
    await tick();
    icalFormEl?.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
  }

  async function removeIcalSub(id: string) {
    await api.ical.deleteSubscription(id).catch(() => {});
    icalSubs = icalSubs.filter(s => s.id !== id);
  }

  async function syncIcalSub(id: string) {
    syncing['ical_' + id] = true;
    try {
      await api.ical.syncSubscription(id);
      icalSubs = await api.ical.listSubscriptions();
    } catch {}
    finally { syncing['ical_' + id] = false; }
  }

  async function syncService(name: string, fn: () => Promise<{ new: number; updated: number; errors: number }>) {
    syncing[name] = true; syncResults[name] = '';
    try {
      const r = await fn();
      syncResults[name] = `${r.new} new, ${r.updated} updated${r.errors ? `, ${r.errors} errors` : ''}`;
    } catch (e) {
      syncResults[name] = 'Error: ' + (e as Error).message;
    } finally { syncing[name] = false; }
  }

  async function connectFastmail() {
    if (!fmEmail.trim() || !fmPassword.trim()) return;
    fmSaving = true; fmError = '';
    try {
      await api.integrations.fastmail.save(fmEmail.trim(), fmPassword.trim());
      fastmail = await api.integrations.fastmail.get();
      fmCal = await api.integrations.fastmail.calendar.get().catch(() => ({ connected: false, enabled: false }));
      caldav = await api.integrations.caldav.get().catch(() => ({ connected: false }));
      fmShowForm = false; fmEmail = ''; fmPassword = '';
    } catch (e) { fmError = (e as Error).message; }
    finally { fmSaving = false; }
  }

  async function connectTaskInbox() {
    if (!tiEmail.trim() || !tiPassword.trim() || !tiAddress.trim()) return;
    tiSaving = true; tiError = '';
    try {
      taskInbox = await api.integrations.taskInbox.save(tiEmail.trim(), tiPassword.trim(), tiAddress.trim());
      tiShowForm = false; tiEmail = ''; tiPassword = '';
    } catch (e) { tiError = (e as Error).message; }
    finally { tiSaving = false; }
  }

  async function addSender() {
    const v = senderInput.trim().toLowerCase();
    if (!v) return;
    const current = taskInbox.allowed_senders ?? [];
    if (current.includes(v)) { senderInput = ''; return; }
    senderSaving = true;
    try {
      const res = await api.integrations.taskInbox.setSenders([...current, v]);
      taskInbox = { ...taskInbox, allowed_senders: res.allowed_senders };
      senderInput = '';
    } finally { senderSaving = false; }
  }

  async function removeSender(s: string) {
    const updated = (taskInbox.allowed_senders ?? []).filter(x => x !== s);
    const res = await api.integrations.taskInbox.setSenders(updated);
    taskInbox = { ...taskInbox, allowed_senders: res.allowed_senders };
  }

  async function toggleCalendar(enabled: boolean) {
    await api.integrations.calendar.toggle(enabled);
    calendar = await api.integrations.calendar.get();
  }

  async function toggleFastmailCalendar(enabled: boolean) {
    syncing['fmcal-toggle'] = true;
    try {
      await api.integrations.fastmail.calendar.toggle(enabled);
      fmCal = { ...fmCal, enabled };
    } catch (e) {
      syncResults['fmcal'] = 'Error: ' + (e as Error).message;
    } finally {
      syncing['fmcal-toggle'] = false;
    }
  }

  async function syncFastmailCalendar() {
    syncing['fmcal'] = true; syncResults['fmcal'] = '';
    try {
      const r = await api.integrations.fastmail.calendar.sync();
      syncResults['fmcal'] = `Synced ${r.synced} events`;
    } catch (e) {
      syncResults['fmcal'] = 'Error: ' + (e as Error).message;
    } finally { syncing['fmcal'] = false; }
  }

  // ── CalDAV: push scheduled tasks to a calendar ──────────────────────────────
  async function openCaldavPicker() {
    caldavPickerOpen = true; caldavError = ''; caldavLoading = true;
    try {
      caldavCalendars = await api.integrations.caldav.calendars();
    } catch (e) {
      caldavError = (e as Error).message;
    } finally { caldavLoading = false; }
  }

  async function selectCaldavCalendar(href: string, name: string) {
    caldavLoading = true; caldavError = '';
    try {
      await api.integrations.caldav.select(href, name);
      caldav = await api.integrations.caldav.get();
      caldavPickerOpen = false;
    } catch (e) {
      caldavError = (e as Error).message;
    } finally { caldavLoading = false; }
  }

  async function toggleCaldav(enabled: boolean) {
    syncing['caldav-toggle'] = true;
    try {
      await api.integrations.caldav.toggle(enabled);
      caldav = { ...caldav, enabled };
    } catch (e) {
      syncResults['caldav'] = 'Error: ' + (e as Error).message;
    } finally { syncing['caldav-toggle'] = false; }
  }

  async function syncCaldav() {
    syncing['caldav'] = true; syncResults['caldav'] = '';
    try {
      const r = await api.integrations.caldav.sync();
      syncResults['caldav'] = `Pushed ${r.synced} tasks`;
      caldav = await api.integrations.caldav.get();
    } catch (e) {
      syncResults['caldav'] = 'Error: ' + (e as Error).message;
    } finally { syncing['caldav'] = false; }
  }

  async function disconnectGmail() {
    if (!confirm('Disconnect Gmail? Imported tasks will be kept.')) return;
    await api.integrations.gmail.delete();
    gmail = { connected: false }; calendar = { connected: false };
  }

  async function disconnectFastmail() {
    if (!confirm('Disconnect Fastmail? Imported tasks will be kept.')) return;
    await api.integrations.fastmail.delete();
    fastmail = { connected: false }; fmCal = { connected: false, enabled: false };
    caldav = { connected: false }; caldavPickerOpen = false;
    taskInbox = { connected: false };
  }

  async function disconnectTaskInbox() {
    if (!confirm('Remove email inbox? Imported tasks will be kept.')) return;
    await api.integrations.taskInbox.delete();
    taskInbox = { connected: false };
  }

  function formatDate(s?: string | null) {
    if (!s) return 'Never';
    return new Date(s).toLocaleString();
  }

  function formatTime(s?: string | null) {
    if (!s) return '';
    return new Date(s).toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' });
  }
</script>

{#snippet sectionIcon(id: string)}
  {#if id === 'integrations'}
    <svg class="h-5 w-5" style="color: var(--sempa-accent);" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24">
      <path stroke-linecap="round" d="M13 10V3L4 14h7v7l9-11h-7z"/>
    </svg>
  {:else if id === 'tasks'}
    <svg class="h-5 w-5" style="color: var(--sempa-accent);" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24">
      <path stroke-linecap="round" d="M9 5H7a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V7a2 2 0 0 0-2-2h-2M9 5a2 2 0 0 0 2 2h2a2 2 0 0 0 2-2M9 5a2 2 0 0 1 2-2h2a2 2 0 0 1 2 2m-6 9 2 2 4-4"/>
    </svg>
  {:else}
    <svg class="h-5 w-5" style="color: var(--sempa-accent);" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24">
      <circle cx="12" cy="12" r="3"/><path stroke-linecap="round" d="M19.07 4.93A10 10 0 1 0 4.93 19.07M12 2v2m0 18v-2m8-8h2M2 12h2m13.66-7.07 1.41-1.41M4.93 19.07l1.41-1.41M19.07 19.07l1.41 1.41M4.93 4.93 3.51 3.51"/>
    </svg>
  {/if}
{/snippet}

{#snippet sectionDesc(id: string)}
  {#if id === 'integrations'}Gmail, Fastmail, Jira, Calendars
  {:else if id === 'tasks'}Tags, recurring templates
  {:else}Accent colour, text size, dark mode
  {/if}
{/snippet}

{#if mobile.value}
  <!-- ══ MOBILE SETTINGS ═══════════════════════════════════════════════════ -->
  {#if !mobileSection}
    <!-- State A: section list -->
    <div style="padding-top: env(safe-area-inset-top, 0px);">
      <div class="px-5 pt-5 pb-4" style="border-bottom: 1px solid var(--sempa-border);">
        <h1 class="text-xl font-bold" style="color: var(--sempa-text);">Settings</h1>
      </div>
      <div class="px-4 py-3">
        {#each MOBILE_SECTIONS as section}
          <button onclick={() => mobileSection = section.id}
                  class="flex w-full items-center gap-4 rounded-xl px-3 py-3.5 transition-colors"
                  style="text-align:left;"
                  onmouseenter={(e) => (e.currentTarget as HTMLElement).style.background = 'var(--sempa-accent-bg)'}
                  onmouseleave={(e) => (e.currentTarget as HTMLElement).style.background = ''}>
            <div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl"
                 style="background: var(--sempa-accent-bg);">
              {@render sectionIcon(section.id)}
            </div>
            <div class="flex-1 min-w-0">
              <p class="text-sm font-semibold" style="color: var(--sempa-text);">{section.label}</p>
              <p class="text-xs" style="color: var(--sempa-text-dim);">{@render sectionDesc(section.id)}</p>
            </div>
            <svg class="h-4 w-4 shrink-0" style="color: var(--sempa-text-dim);" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
              <path stroke-linecap="round" d="m9 18 6-6-6-6"/>
            </svg>
          </button>
        {/each}
      </div>

      <!-- Theme toggle -->
      <div class="mx-4 mt-1 px-3 py-3 rounded-xl" style="border: 1px solid var(--sempa-border);">
        <div class="flex items-center justify-between">
          <p class="text-sm font-medium" style="color: var(--sempa-text);">
            {theme.dark ? 'Dark mode' : 'Light mode'}
          </p>
          <button onclick={theme.toggle}
                  class="relative inline-flex h-6 w-11 items-center rounded-full transition-colors"
                  style="background: {theme.dark ? 'var(--sempa-accent)' : 'var(--sempa-border)'};">
            <span class="inline-block h-4 w-4 rounded-full bg-white shadow transition-transform"
                  style="transform: translateX({theme.dark ? '24px' : '4px'});"></span>
          </button>
        </div>
      </div>

      <!-- Version -->
      <p class="mt-8 text-center text-[11px] pb-6" style="color: var(--sempa-text-dim);">Sempa · self-hosted</p>
    </div>

  {:else}
    <!-- State B: section detail -->
    <div style="padding-top: env(safe-area-inset-top, 0px); display:flex; flex-direction:column; height:100%;">
      <!-- Back header -->
      <div class="flex items-center gap-3 px-4 py-3" style="border-bottom: 1px solid var(--sempa-border);">
        <button onclick={() => mobileSection = null}
                class="flex items-center gap-1.5 rounded-lg px-2 py-1.5 text-sm transition-colors"
                style="color: var(--sempa-accent);">
          <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
            <path stroke-linecap="round" d="M19 12H5m7-7-7 7 7 7"/>
          </svg>
          Settings
        </button>
        <p class="text-sm font-semibold" style="color: var(--sempa-text);">
          {NAV_SECTIONS.find(s => s.id === mobileSection)?.label}
        </p>
      </div>
      <!-- Section content -->
      <div class="flex-1 overflow-y-auto">
        <div class="px-4 py-4 pb-16">
          {#if mobileSection === 'integrations'}
            {@render integrationsContent()}
          {:else if mobileSection === 'tasks'}
            {@render tasksContent()}
          {:else if mobileSection === 'appearance'}
            {@render appearanceContent()}
          {/if}
        </div>
      </div>
    </div>
  {/if}

{:else}
  <!-- ══ DESKTOP SETTINGS ══════════════════════════════════════════════════ -->
  <div class="flex h-full">

    <!-- ── Settings sub-nav ──────────────────────────────────────────────── -->
    <nav class="flex w-[148px] shrink-0 flex-col gap-1 px-3 pt-8"
         style="background: var(--sempa-bg-nav); border-right: 1px solid var(--sempa-border);">
      <h1 class="mb-4 px-3 text-base font-semibold" style="color: var(--sempa-text);">Settings</h1>
      {#each NAV_SECTIONS as section}
        <button onclick={() => scrollTo(section.id)}
                class="rounded-lg px-3 py-2 text-left text-[13px] font-medium transition-colors"
                style={activeSection === section.id
                  ? 'background: var(--sempa-accent-bg); color: var(--sempa-accent);'
                  : 'color: var(--sempa-text-soft);'}>
          {section.label}
        </button>
      {/each}
    </nav>

    <!-- ── Scrollable content ────────────────────────────────────────────── -->
    <div bind:this={scrollContainer} class="flex-1 overflow-y-auto">
      <div class="mx-auto max-w-xl px-6 py-8 pb-16">
        {@render integrationsContent()}
        {@render tasksContent()}
        {@render appearanceContent()}
      </div>
    </div>
  </div>
{/if}

<!-- ══ CONTENT SNIPPETS (accessible from both mobile and desktop) ══════════ -->

{#snippet integrationsContent()}
  <!-- ═══════════════════════════════════════════════════════════════════════
       SECTION: Integrations
  ════════════════════════════════════════════════════════════════════════ -->
  <div id="settings-integrations">

    {#if serverUnreachable}
      <div class="mb-4 rounded-xl px-4 py-3 text-sm"
           style="border: 1px solid var(--sempa-amber); background: color-mix(in srgb, var(--sempa-amber) 10%, var(--sempa-bg-panel)); color: var(--sempa-amber);">
        <p class="font-semibold">Can't reach your server</p>
        <p class="mt-1 text-xs leading-relaxed" style="color: var(--sempa-text-soft);">
          Integration status couldn't load. Check your connection and that the server URL is
          correct, then pull to refresh or reopen this screen.
        </p>
      </div>
    {/if}

    <!-- ── Email & Calendar ──────────────────────────────────────── -->
    <p class="mb-3" style="font-family:monospace; font-size:10px; font-weight:700; letter-spacing:0.12em;
     text-transform:uppercase; color:var(--sempa-text-dim)">Email & Calendar</p>

    <!-- ── Gmail ─────────────────────────────────────────────────── -->
    <section class="mb-3 overflow-hidden rounded-xl border" style="border-color: var(--sempa-border); background: var(--sempa-bg-panel);">
      <!-- Header -->
      <div class="flex items-center gap-3 px-5 py-4"
           class:border-b={gmail.connected}
           style="border-color: var(--sempa-border);">
        <div class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-red-50 dark:bg-red-950">
          <svg class="h-4 w-4 text-red-500" viewBox="0 0 24 24" fill="currentColor">
            <path d="M20 4H4c-1.1 0-2 .9-2 2v12c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V6c0-1.1-.9-2-2-2zm0 4-8 5-8-5V6l8 5 8-5v2z"/>
          </svg>
        </div>
        <div class="flex-1 min-w-0">
          <p class="text-sm font-semibold" style="color: var(--sempa-text);">Gmail</p>
          {#if gmail.connected}
            <p class="text-xs truncate" style="color: var(--sempa-text-soft);">
              {gmail.email} &middot; Last synced {formatTime(gmail.last_synced_at)}
            </p>
          {:else}
            <p class="text-xs" style="color: var(--sempa-text-dim);">Import starred emails as tasks</p>
          {/if}
        </div>
        {#if gmail.connected}
          <span class="inline-flex items-center gap-1.5 text-xs font-medium" style="color: var(--sempa-text-soft);">
            <span class="h-1.5 w-1.5 rounded-full bg-green-500"></span>Connected
          </span>
        {:else}
          <a href={api.integrations.gmail.authUrl(false)}
             class="rounded-lg border px-3 py-1.5 text-xs font-medium transition-colors"
             style="border-color: var(--sempa-border); color: var(--sempa-text-soft);">
            Connect &rarr;
          </a>
        {/if}
      </div>

      {#if gmail.connected}
        <!-- Sub-features -->
        <div class="flex h-10 items-center justify-between border-b px-5 text-xs"
             style="border-color: var(--sempa-border);">
          <span style="color: var(--sempa-text-soft);">Starred emails</span>
          <div class="flex items-center gap-3">
            {#if syncResults['gmail']}
              <span style="color: var(--sempa-accent);">{syncResults['gmail']}</span>
            {/if}
            <button onclick={() => syncService('gmail', api.integrations.gmail.sync)}
                    disabled={syncing['gmail']}
                    class="transition-colors disabled:opacity-50"
                    style="color: var(--sempa-text-dim);">
              {syncing['gmail'] ? 'Syncing...' : 'Sync'}
            </button>
          </div>
        </div>

        <!-- Google Calendar sub-feature -->
        <div class="flex h-10 items-center justify-between border-b px-5 text-xs"
             style="border-color: var(--sempa-border);">
          <span style="color: var(--sempa-text-soft);">Google Calendar</span>
          <div class="flex items-center gap-3">
            {#if syncResults['calendar']}
              <span style="color: var(--sempa-accent);">{syncResults['calendar']}</span>
            {/if}
            {#if calendar.connected}
              <button onclick={() => syncService('calendar', () => api.integrations.calendar.sync())}
                      disabled={syncing['calendar']}
                      class="transition-colors disabled:opacity-50"
                      style="color: var(--sempa-text-dim);">
                {syncing['calendar'] ? 'Syncing...' : 'Sync today'}
              </button>
              <button onclick={() => toggleCalendar(false)}
                      aria-label="Disable Google Calendar"
                      class="relative inline-flex h-5 w-9 items-center rounded-full transition-colors"
                      style="background: var(--sempa-accent);">
                <span class="inline-block h-3.5 w-3.5 rounded-full bg-white shadow transition-transform"
                      style="transform: translateX(18px);"></span>
              </button>
            {:else}
              <a href={api.integrations.gmail.authUrl(true)}
                 class="transition-colors"
                 style="color: var(--sempa-accent);">
                Enable
              </a>
            {/if}
          </div>
        </div>

        <!-- Gmail labels link -->
        <div class="flex h-10 items-center justify-between border-b px-5 text-xs"
             style="border-color: var(--sempa-border);">
          <span style="color: var(--sempa-text-soft);">Label filters</span>
          <a href="/settings/integrations/gmail" style="color: var(--sempa-text-dim);">
            Configure &rarr;
          </a>
        </div>

        <!-- Disconnect -->
        <div class="px-5 py-3">
          <button onclick={disconnectGmail} class="text-xs transition-colors" style="color: var(--sempa-text-dim);">
            Disconnect Gmail
          </button>
        </div>
      {/if}
    </section>

    <!-- ── Fastmail ──────────────────────────────────────────────── -->
    <section class="mb-3 overflow-hidden rounded-xl border" style="border-color: var(--sempa-border); background: var(--sempa-bg-panel);">
      <!-- Header -->
      <div class="flex items-center gap-3 px-5 py-4"
           class:border-b={fastmail.connected || fmShowForm}
           style="border-color: var(--sempa-border);">
        <div class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg" style="background: var(--sempa-accent-bg);">
          <svg class="h-4 w-4" style="color: var(--sempa-accent);" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24">
            <path stroke-linecap="round" d="M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"/>
          </svg>
        </div>
        <div class="flex-1 min-w-0">
          <p class="text-sm font-semibold" style="color: var(--sempa-text);">Fastmail</p>
          {#if fastmail.connected}
            <p class="text-xs truncate" style="color: var(--sempa-text-soft);">
              {fastmail.email} &middot; Last synced {formatTime(fastmail.last_synced_at)}
            </p>
          {:else}
            <p class="text-xs" style="color: var(--sempa-text-dim);">Sync starred emails and calendar via JMAP</p>
          {/if}
        </div>
        {#if fastmail.connected}
          <span class="inline-flex items-center gap-1.5 text-xs font-medium" style="color: var(--sempa-text-soft);">
            <span class="h-1.5 w-1.5 rounded-full bg-green-500"></span>Connected
          </span>
        {:else if !fmShowForm}
          <button onclick={() => fmShowForm = true}
                  class="rounded-lg border px-3 py-1.5 text-xs font-medium transition-colors"
                  style="border-color: var(--sempa-border); color: var(--sempa-text-soft);">
            Connect &rarr;
          </button>
        {/if}
      </div>

      {#if fastmail.connected}
        <!-- Starred emails -->
        <div class="flex h-10 items-center justify-between border-b px-5 text-xs"
             style="border-color: var(--sempa-border);">
          <span style="color: var(--sempa-text-soft);">Starred emails</span>
          <div class="flex items-center gap-3">
            {#if syncResults['fastmail']}
              <span style="color: var(--sempa-accent);">{syncResults['fastmail']}</span>
            {/if}
            <button onclick={() => syncService('fastmail', api.integrations.fastmail.sync)}
                    disabled={syncing['fastmail']}
                    class="transition-colors disabled:opacity-50"
                    style="color: var(--sempa-text-dim);">
              {syncing['fastmail'] ? 'Syncing...' : 'Sync'}
            </button>
          </div>
        </div>

        <!-- Fastmail Calendar -->
        {#if fmCal.connected}
          <div class="flex h-10 items-center justify-between border-b px-5 text-xs"
               style="border-color: var(--sempa-border);">
            <span style="color: var(--sempa-text-soft);">
              Calendar
              {#if fmCal.enabled && fmCal.last_synced_at}
                <span style="color: var(--sempa-text-dim);"> &middot; {formatTime(fmCal.last_synced_at)}</span>
              {/if}
            </span>
            <div class="flex items-center gap-3">
              {#if syncResults['fmcal']}
                <span style="color: var(--sempa-accent);">{syncResults['fmcal']}</span>
              {/if}
              {#if fmCal.enabled}
                <button onclick={syncFastmailCalendar}
                        disabled={syncing['fmcal']}
                        class="transition-colors disabled:opacity-50"
                        style="color: var(--sempa-text-dim);">
                  {syncing['fmcal'] ? 'Syncing...' : 'Sync'}
                </button>
              {/if}
              <button onclick={() => toggleFastmailCalendar(!fmCal.enabled)}
                      disabled={syncing['fmcal-toggle']}
                      aria-label="Toggle Fastmail Calendar"
                      class="relative inline-flex h-5 w-9 items-center rounded-full transition-colors disabled:opacity-50"
                      style="background: {fmCal.enabled ? 'var(--sempa-accent)' : 'var(--sempa-border)'};">
                <span class="inline-block h-3.5 w-3.5 rounded-full bg-white shadow transition-transform"
                      style="transform: translateX({fmCal.enabled ? '18px' : '3px'});"></span>
              </button>
            </div>
          </div>
        {/if}

        <!-- CalDAV — push scheduled tasks to a calendar -->
        {#if caldav.connected}
          <div class="border-b px-5 py-2.5 text-xs" style="border-color: var(--sempa-border);">
            <div class="flex min-h-7 items-center justify-between">
              <span style="color: var(--sempa-text-soft);">
                Push tasks to calendar
                {#if caldav.calendar_name}
                  <span style="color: var(--sempa-text-dim);"> &middot; {caldav.calendar_name}</span>
                {/if}
                {#if caldav.enabled && caldav.last_synced_at}
                  <span style="color: var(--sempa-text-dim);"> &middot; {formatTime(caldav.last_synced_at)}</span>
                {/if}
              </span>
              <div class="flex items-center gap-3">
                {#if syncResults['caldav']}
                  <span style="color: var(--sempa-accent);">{syncResults['caldav']}</span>
                {/if}
                {#if caldav.calendar_href}
                  {#if caldav.enabled}
                    <button onclick={syncCaldav} disabled={syncing['caldav']}
                            class="transition-colors disabled:opacity-50"
                            style="color: var(--sempa-text-dim);">
                      {syncing['caldav'] ? 'Pushing...' : 'Sync now'}
                    </button>
                  {/if}
                  <button onclick={openCaldavPicker} disabled={caldavLoading}
                          class="transition-colors disabled:opacity-50"
                          style="color: var(--sempa-text-dim);">Change</button>
                  <button onclick={() => toggleCaldav(!caldav.enabled)}
                          disabled={syncing['caldav-toggle']}
                          aria-label="Toggle CalDAV task push"
                          class="relative inline-flex h-5 w-9 items-center rounded-full transition-colors disabled:opacity-50"
                          style="background: {caldav.enabled ? 'var(--sempa-accent)' : 'var(--sempa-border)'};">
                    <span class="inline-block h-3.5 w-3.5 rounded-full bg-white shadow transition-transform"
                          style="transform: translateX({caldav.enabled ? '18px' : '3px'});"></span>
                  </button>
                {:else}
                  <button onclick={openCaldavPicker} disabled={caldavLoading}
                          class="rounded-lg border px-3 py-1 font-medium transition-colors disabled:opacity-50"
                          style="border-color: var(--sempa-border); color: var(--sempa-text-soft);">
                    {caldavLoading ? 'Loading...' : 'Set up'}
                  </button>
                {/if}
              </div>
            </div>

            {#if caldavPickerOpen}
              <div class="mt-2.5 space-y-1.5">
                {#if caldavError}
                  <p class="text-red-600 dark:text-red-400">{caldavError}</p>
                {/if}
                {#if caldavLoading}
                  <p style="color: var(--sempa-text-dim);">Loading calendars…</p>
                {:else if caldavCalendars.length === 0 && !caldavError}
                  <p style="color: var(--sempa-text-dim);">No writable calendars found.</p>
                {:else}
                  <p class="mb-1" style="color: var(--sempa-text-dim);">Choose a calendar to write task time-blocks into:</p>
                  {#each caldavCalendars as cal (cal.href)}
                    {@const selected = caldav.calendar_href === cal.href}
                    <button onclick={() => selectCaldavCalendar(cal.href, cal.name)}
                            disabled={caldavLoading}
                            class="flex w-full items-center gap-2 rounded-lg border px-3 py-2 text-left transition-colors disabled:opacity-50"
                            style="border-color: {selected ? 'var(--sempa-accent)' : 'var(--sempa-border)'};
                                   background: {selected ? 'var(--sempa-accent-bg)' : 'transparent'};">
                      <span class="h-2.5 w-2.5 shrink-0 rounded-full"
                            style="background: {cal.color || '#6b7280'};"></span>
                      <span class="flex-1 min-w-0 truncate" style="color: var(--sempa-text-soft);">{cal.name}</span>
                      {#if selected}
                        <span style="color: var(--sempa-accent);">Selected</span>
                      {/if}
                    </button>
                  {/each}
                {/if}
                <button onclick={() => { caldavPickerOpen = false; caldavError = ''; }}
                        class="transition-colors" style="color: var(--sempa-text-dim);">Cancel</button>
              </div>
            {/if}
          </div>
        {/if}

        <!-- Email Inbox (Fastmail sub-feature) -->
        {#if taskInbox.connected}
          <div class="flex h-10 items-center justify-between border-b px-5 text-xs"
               style="border-color: var(--sempa-border);">
            <span style="color: var(--sempa-text-soft);">
              Email Inbox
              {#if taskInbox.inbox_address}
                <span class="font-mono" style="color: var(--sempa-text-dim);"> &middot; {taskInbox.inbox_address}</span>
              {/if}
            </span>
            <div class="flex items-center gap-3">
              {#if syncResults['task-inbox']}
                <span style="color: var(--sempa-accent);">{syncResults['task-inbox']}</span>
              {/if}
              <button onclick={() => syncService('task-inbox', api.integrations.taskInbox.sync)}
                      disabled={syncing['task-inbox']}
                      class="transition-colors disabled:opacity-50"
                      style="color: var(--sempa-text-dim);">
                {syncing['task-inbox'] ? 'Syncing...' : 'Sync'}
              </button>
            </div>
          </div>

          <!-- Allowed senders -->
          <div class="border-b px-5 py-3 space-y-2" style="border-color: var(--sempa-border);">
            <div class="flex items-center justify-between">
              <p class="text-xs font-medium" style="color: var(--sempa-text-soft);">Allowed senders</p>
              {#if (taskInbox.allowed_senders ?? []).length === 0}
                <p class="text-xs" style="color: var(--sempa-text-dim);">All senders allowed</p>
              {/if}
            </div>
            {#if (taskInbox.allowed_senders ?? []).length > 0}
              <div class="flex flex-wrap gap-1.5">
                {#each (taskInbox.allowed_senders ?? []) as sender}
                  <span class="inline-flex items-center gap-1 rounded-full border px-2.5 py-1 text-xs"
                        style="border-color: var(--sempa-border); color: var(--sempa-text-soft);">
                    {sender}
                    <button onclick={() => removeSender(sender)} aria-label="Remove {sender}" class="hover:text-red-500 transition-colors">
                      <svg class="h-3 w-3" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                        <path stroke-linecap="round" d="M6 18L18 6M6 6l12 12"/>
                      </svg>
                    </button>
                  </span>
                {/each}
              </div>
            {/if}
            <form onsubmit={(e) => { e.preventDefault(); addSender(); }} class="flex gap-2">
              <input bind:value={senderInput}
                     placeholder="@company.com or user@example.com"
                     class="flex-1 rounded-lg border px-3 py-1.5 text-xs outline-none"
                     style="border-color: var(--sempa-border); background: var(--sempa-bg-main); color: var(--sempa-text);" />
              <button type="submit" disabled={senderSaving || !senderInput.trim()}
                      class="rounded-lg border px-3 py-1.5 text-xs font-medium transition-colors disabled:opacity-40"
                      style="border-color: var(--sempa-border); color: var(--sempa-text-soft);">
                Add
              </button>
            </form>
          </div>

          <!-- Disconnect inbox -->
          <div class="flex h-10 items-center justify-between border-b px-5 text-xs"
               style="border-color: var(--sempa-border);">
            <button onclick={disconnectTaskInbox} class="transition-colors" style="color: var(--sempa-text-dim);">
              Remove email inbox
            </button>
          </div>
        {:else}
          <!-- Email inbox not set up yet -->
          <div class="flex h-10 items-center justify-between border-b px-5 text-xs"
               style="border-color: var(--sempa-border);">
            <span style="color: var(--sempa-text-soft);">Email Inbox &mdash; forward emails to create tasks</span>
            <button onclick={() => tiShowForm = true}
                    class="transition-colors"
                    style="color: var(--sempa-accent);">
              Set up &rarr;
            </button>
          </div>
        {/if}

        <!-- Email inbox setup form (inline) -->
        {#if tiShowForm && !taskInbox.connected}
          <div class="border-b px-5 py-4 space-y-3" style="border-color: var(--sempa-border);">
            <div>
              <label class="mb-1 block text-xs font-medium" style="color: var(--sempa-text-soft);" for="ti-email">Fastmail email</label>
              <input id="ti-email" type="email" inputmode="email" autocapitalize="none" spellcheck="false" bind:value={tiEmail} placeholder="you@fastmail.com"
                     class="w-full rounded-lg border px-3 py-2 text-sm outline-none"
                     style="border-color: var(--sempa-border); background: var(--sempa-bg-main); color: var(--sempa-text);" />
            </div>
            <div>
              <label class="mb-1 block text-xs font-medium" style="color: var(--sempa-text-soft);" for="ti-pass">App password</label>
              <input id="ti-pass" type="password" bind:value={tiPassword}
                     placeholder="Generate at Fastmail -> Settings -> Privacy & Security"
                     class="w-full rounded-lg border px-3 py-2 text-sm outline-none"
                     style="border-color: var(--sempa-border); background: var(--sempa-bg-main); color: var(--sempa-text);" />
            </div>
            <div>
              <label class="mb-1 block text-xs font-medium" style="color: var(--sempa-text-soft);" for="ti-addr">Forwarding address</label>
              <input id="ti-addr" type="email" inputmode="email" autocapitalize="none" spellcheck="false" bind:value={tiAddress} placeholder="tasks@sempa.ca"
                     class="w-full rounded-lg border px-3 py-2 text-sm outline-none"
                     style="border-color: var(--sempa-border); background: var(--sempa-bg-main); color: var(--sempa-text);" />
            </div>
            {#if tiError}<p class="text-sm text-red-600 dark:text-red-400">{tiError}</p>{/if}
            <div class="flex gap-2">
              <button onclick={connectTaskInbox} disabled={tiSaving || !tiEmail || !tiPassword || !tiAddress}
                      class="rounded-lg px-4 py-2 text-sm font-medium text-white disabled:opacity-40"
                      style="background: var(--sempa-accent);">
                {tiSaving ? 'Connecting...' : 'Connect'}
              </button>
              <button onclick={() => { tiShowForm = false; tiError = ''; }}
                      class="rounded-lg border px-4 py-2 text-sm transition-colors"
                      style="border-color: var(--sempa-border); color: var(--sempa-text-soft);">
                Cancel
              </button>
            </div>
          </div>
        {/if}

        <!-- Disconnect Fastmail -->
        <div class="px-5 py-3">
          <button onclick={disconnectFastmail} class="text-xs transition-colors" style="color: var(--sempa-text-dim);">
            Disconnect Fastmail
          </button>
        </div>

      {:else if fmShowForm}
        <!-- Fastmail connect form -->
        <div class="px-5 py-4 space-y-3">
          <div>
            <label class="mb-1 block text-xs font-medium" style="color: var(--sempa-text-soft);" for="fm-email">Email</label>
            <input id="fm-email" type="email" inputmode="email" autocapitalize="none" spellcheck="false" bind:value={fmEmail} placeholder="you@fastmail.com"
                   class="w-full rounded-lg border px-3 py-2 text-sm outline-none"
                   style="border-color: var(--sempa-border); background: var(--sempa-bg-main); color: var(--sempa-text);" />
          </div>
          <div>
            <label class="mb-1 block text-xs font-medium" style="color: var(--sempa-text-soft);" for="fm-pass">App Password</label>
            <input id="fm-pass" type="password" bind:value={fmPassword}
                   placeholder="Generate at fastmail.com -> Settings -> Security"
                   class="w-full rounded-lg border px-3 py-2 text-sm outline-none"
                   style="border-color: var(--sempa-border); background: var(--sempa-bg-main); color: var(--sempa-text);" />
            <p class="mt-1 text-xs" style="color: var(--sempa-text-dim);">
              Create at fastmail.com -> Settings -> Privacy & Security -> App Passwords
            </p>
          </div>
          {#if fmError}<p class="text-sm text-red-600 dark:text-red-400">{fmError}</p>{/if}
          <div class="flex gap-2">
            <button onclick={connectFastmail} disabled={fmSaving || !fmEmail || !fmPassword}
                    class="rounded-lg px-4 py-2 text-sm font-medium text-white disabled:opacity-40"
                    style="background: var(--sempa-accent);">
              {fmSaving ? 'Connecting...' : 'Connect'}
            </button>
            <button onclick={() => { fmShowForm = false; fmError = ''; }}
                    class="rounded-lg border px-4 py-2 text-sm transition-colors"
                    style="border-color: var(--sempa-border); color: var(--sempa-text-soft);">
              Cancel
            </button>
          </div>
        </div>
      {/if}
    </section>

    <!-- ── Calendar Feeds (ICS) ──────────────────────────────────── -->
    <section class="mb-8 overflow-hidden rounded-xl border" style="border-color: var(--sempa-border); background: var(--sempa-bg-panel);">
      <!-- Header -->
      <div class="flex items-center gap-3 px-5 py-4"
           class:border-b={icalSubs.length > 0 || showIcalForm}
           style="border-color: var(--sempa-border);">
        <div class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg"
             style="background: var(--sempa-cal-feed-bg, #1a2820);">
          <svg class="h-4 w-4" style="color: var(--sempa-success);" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24">
            <rect x="3" y="4" width="18" height="18" rx="2" ry="2"/>
            <line x1="16" y1="2" x2="16" y2="6"/>
            <line x1="8" y1="2" x2="8" y2="6"/>
            <line x1="3" y1="10" x2="21" y2="10"/>
          </svg>
        </div>
        <div class="flex-1 min-w-0">
          <p class="text-sm font-semibold" style="color: var(--sempa-text);">Calendar Feeds</p>
          <p class="text-xs" style="color: var(--sempa-text-dim);">Subscribe to ICS/webcal URLs</p>
        </div>
        <button onclick={openIcalForm}
                class="rounded-lg border px-3 py-1.5 text-xs font-medium transition-colors"
                style="border-color: var(--sempa-border); color: var(--sempa-text-soft);">
          + Add feed
        </button>
      </div>

      {#if icalSubs.length === 0 && !showIcalForm}
        <div class="border-t px-5 py-3" style="border-color: var(--sempa-border);">
          <p class="text-xs" style="color: var(--sempa-text-dim);">No feeds added yet.</p>
        </div>
      {/if}

      {#each icalSubs as sub (sub.id)}
        <div class="flex h-10 items-center gap-3 border-t px-5 text-xs"
             style="border-color: var(--sempa-border);">
          <div class="h-2.5 w-2.5 shrink-0 rounded-full" style="background:{sub.color}"></div>
          <span class="flex-1 min-w-0 truncate" style="color: var(--sempa-text-soft);">{sub.name}</span>
          {#if sub.error_msg}
            <span class="text-red-500 dark:text-red-400 truncate max-w-[120px]" title={sub.error_msg}>Error</span>
          {:else if sub.last_synced_at}
            <span style="color: var(--sempa-text-dim);">{formatTime(sub.last_synced_at)}</span>
          {/if}
          <button onclick={() => syncIcalSub(sub.id)} disabled={syncing['ical_' + sub.id]}
                  class="transition-colors disabled:opacity-40"
                  style="color: var(--sempa-text-dim);">
            {syncing['ical_' + sub.id] ? '...' : 'Sync'}
          </button>
          <button onclick={() => removeIcalSub(sub.id)} aria-label="Remove feed"
                  class="transition-colors" style="color: var(--sempa-text-dim);">
            <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
              <path stroke-linecap="round" d="M6 18L18 6M6 6l12 12"/>
            </svg>
          </button>
        </div>
      {/each}

      {#if showIcalForm}
        <div bind:this={icalFormEl}
             class="border-t px-5 py-4 space-y-3" style="border-color: var(--sempa-border);">
          <div>
            <label class="mb-1 block text-xs font-medium" style="color: var(--sempa-text-soft);" for="ical-url">
              ICS / Webcal URL <span class="text-red-400">*</span>
            </label>
            <input id="ical-url" type="url" inputmode="url" bind:value={icalUrl}
                   autocomplete="off" autocapitalize="none" spellcheck="false"
                   placeholder="https://example.com/calendar.ics  or  webcal://..."
                   class="w-full rounded-lg border px-3 py-2 text-sm outline-none"
                   style="border-color: var(--sempa-border); background: var(--sempa-bg-main); color: var(--sempa-text);" />
          </div>
          <div class="grid grid-cols-2 gap-3">
            <div>
              <label class="mb-1 block text-xs font-medium" style="color: var(--sempa-text-soft);" for="ical-name">Name (optional)</label>
              <input id="ical-name" type="text" bind:value={icalName}
                     placeholder="Work calendar"
                     class="w-full rounded-lg border px-3 py-2 text-sm outline-none"
                     style="border-color: var(--sempa-border); background: var(--sempa-bg-main); color: var(--sempa-text);" />
            </div>
            <div>
              <label class="mb-1 block text-xs font-medium" style="color: var(--sempa-text-soft);" for="ical-color">Colour</label>
              <div class="flex flex-wrap items-center gap-2" id="ical-color">
                {#each ['#ef4444','#f97316','#eab308','#22c55e','#14b8a6','#3b82f6','#8b5cf6','#ec4899','#cc6e3a','#b3592e','#6b7280','#f0ece4'] as swatch}
                  {@const isSel = icalColor.toLowerCase() === swatch.toLowerCase()}
                  <button type="button" onclick={() => icalColor = swatch}
                          aria-label="Select colour {swatch}"
                          class="h-7 w-7 rounded-full border-2 transition-transform hover:scale-110"
                          style="background: {swatch}; border-color: {isSel ? 'var(--sempa-accent)' : 'transparent'};
                                 {isSel ? 'outline: 2px solid var(--sempa-accent); outline-offset: 1px;' : ''}">
                  </button>
                {/each}
              </div>
              <span class="mt-1.5 block text-xs font-mono" style="color: var(--sempa-text-dim);">{icalColor}</span>
            </div>
          </div>
          {#if icalError}<p class="text-sm text-red-600 dark:text-red-400">{icalError}</p>{/if}
          <div class="flex gap-2">
            <button onclick={addIcalSub} disabled={icalAdding || !icalUrl.trim()}
                    class="rounded-lg px-4 py-2 text-sm font-medium text-white disabled:opacity-40 transition-colors"
                    style="background: var(--sempa-accent);">
              {icalAdding ? 'Adding...' : 'Subscribe'}
            </button>
            <button onclick={() => { showIcalForm = false; icalError = ''; }}
                    class="rounded-lg border px-4 py-2 text-sm transition-colors"
                    style="border-color: var(--sempa-border); color: var(--sempa-text-soft);">
              Cancel
            </button>
          </div>
        </div>
      {/if}
    </section>

    <!-- ── Project Management ────────────────────────────────────── -->
    <p class="mb-3" style="font-family:monospace; font-size:10px; font-weight:700; letter-spacing:0.12em;
     text-transform:uppercase; color:var(--sempa-text-dim)">Project Management</p>

    <div class="mb-8">
      <a href="/settings/integrations/jira"
         class="flex items-center gap-3 overflow-hidden rounded-xl border px-5 py-4 transition-colors"
         style="border-color: var(--sempa-border); background: var(--sempa-bg-panel);">
        <div class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg" style="background: var(--sempa-accent-bg);">
          <svg class="h-4 w-4" style="color: var(--sempa-accent);" viewBox="0 0 24 24" fill="currentColor">
            <path d="M11.571 11.513H0a5.218 5.218 0 0 0 5.232 5.215h2.13v2.057A5.215 5.215 0 0 0 12.575 24V12.518a1.005 1.005 0 0 0-1.005-1.005zm5.723-5.756H5.757a5.215 5.215 0 0 0 5.214 5.214h2.129v2.058A5.218 5.218 0 0 0 18.313 18.3V6.763a1.006 1.006 0 0 0-1.019-1.006zM23.277.007H11.749a5.215 5.215 0 0 0 5.214 5.214h2.129v2.058A5.218 5.218 0 0 0 24.282 12.5V1.012A1.005 1.005 0 0 0 23.277.007z"/>
          </svg>
        </div>
        <div class="flex-1 min-w-0">
          <p class="text-sm font-semibold" style="color: var(--sempa-text);">Jira</p>
          {#if jira.connected}
            <p class="text-xs" style="color: var(--sempa-text-soft);">Connected &middot; syncs assigned issues</p>
          {:else}
            <p class="text-xs" style="color: var(--sempa-text-dim);">Sync assigned Jira issues as tasks</p>
          {/if}
        </div>
        {#if jira.connected}
          <span class="inline-flex items-center gap-1.5 text-xs font-medium" style="color: var(--sempa-text-soft);">
            <span class="h-1.5 w-1.5 rounded-full bg-green-500"></span>Connected
          </span>
        {:else}
          <span class="text-xs" style="color: var(--sempa-text-dim);">Connect &rarr;</span>
        {/if}
      </a>
    </div>
  </div>
{/snippet}

{#snippet tasksContent()}
  <!-- ═══════════════════════════════════════════════════════════════════════
       SECTION: Tasks
  ════════════════════════════════════════════════════════════════════════ -->
  <div id="settings-tasks">
    <p class="mb-3" style="font-family:monospace; font-size:10px; font-weight:700; letter-spacing:0.12em;
     text-transform:uppercase; color:var(--sempa-text-dim)">Tasks</p>

    <div class="mb-8 flex flex-col gap-2">
      <a href="/settings/tags"
         class="flex items-center gap-3 rounded-xl border px-5 py-4 transition-colors"
         style="border-color: var(--sempa-border); background: var(--sempa-bg-panel);">
        <div class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-violet-50 dark:bg-violet-950">
          <svg class="h-4 w-4 text-violet-500" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24">
            <path stroke-linecap="round" d="M7 7h.01M7 3h5c.512 0 1.024.195 1.414.586l7 7a2 2 0 0 1 0 2.828l-7 7a2 2 0 0 1-2.828 0l-7-7A2 2 0 0 1 3 12V7a4 4 0 0 1 4-4z"/>
          </svg>
        </div>
        <div class="flex-1">
          <p class="text-sm font-semibold" style="color: var(--sempa-text);">Tags</p>
          <p class="text-xs" style="color: var(--sempa-text-soft);">Colour-coded labels for your tasks</p>
        </div>
        <svg class="h-4 w-4" style="color: var(--sempa-text-dim);" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
          <path stroke-linecap="round" d="m9 18 6-6-6-6"/>
        </svg>
      </a>

      <a href="/settings/recurring"
         class="flex items-center gap-3 rounded-xl border px-5 py-4 transition-colors"
         style="border-color: var(--sempa-border); background: var(--sempa-bg-panel);">
        <div class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg" style="background: var(--sempa-accent-bg);">
          <svg class="h-4 w-4" style="color: var(--sempa-accent);" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"/>
          </svg>
        </div>
        <div class="flex-1">
          <p class="text-sm font-semibold" style="color: var(--sempa-text);">Recurring Tasks</p>
          <p class="text-xs" style="color: var(--sempa-text-soft);">Daily, weekly, and monthly templates</p>
        </div>
        <svg class="h-4 w-4" style="color: var(--sempa-text-dim);" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
          <path stroke-linecap="round" d="m9 18 6-6-6-6"/>
        </svg>
      </a>

      <a href="/settings/backup"
         class="flex items-center gap-3 rounded-xl border px-5 py-4 transition-colors"
         style="border-color: var(--sempa-border); background: var(--sempa-bg-panel);">
        <div class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-sky-50 dark:bg-sky-950">
          <svg class="h-4 w-4 text-sky-500" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" d="M3 7v10a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2V7M3 7l2-3h14l2 3M3 7h18M9 12h6"/>
          </svg>
        </div>
        <div class="flex-1">
          <p class="text-sm font-semibold" style="color: var(--sempa-text);">Backup &amp; Restore</p>
          <p class="text-xs" style="color: var(--sempa-text-soft);">Automatic backups, encryption, and recovery</p>
        </div>
        <svg class="h-4 w-4" style="color: var(--sempa-text-dim);" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
          <path stroke-linecap="round" d="m9 18 6-6-6-6"/>
        </svg>
      </a>
    </div>
  </div>
{/snippet}

{#snippet appearanceContent()}
  <!-- ═══════════════════════════════════════════════════════════════════════
       SECTION: Appearance
  ════════════════════════════════════════════════════════════════════════ -->
  <div id="settings-appearance">
    <p class="mb-3" style="font-family:monospace; font-size:10px; font-weight:700; letter-spacing:0.12em;
     text-transform:uppercase; color:var(--sempa-text-dim)">Appearance</p>

    <section class="overflow-hidden rounded-xl border" style="border-color: var(--sempa-border); background: var(--sempa-bg-panel);">
      <div class="px-5 py-5 space-y-6">

        <!-- Accent colour -->
        <div>
          <p class="mb-3 text-xs font-medium" style="color: var(--sempa-text-soft);">Accent colour</p>
          <div style="display:grid; grid-template-columns:repeat(auto-fill,28px); gap:8px;">
            {#each Object.entries(ACCENT_PRESETS) as [name, preset]}
              <button onclick={() => theme.setAccent(name as AccentName)}
                      title={preset.label}
                      class="transition-transform hover:scale-110"
                      style="width:28px; height:28px; border-radius:14px; border:none; cursor:pointer;
                             background: {preset.swatch};
                             {theme.accent === name
                               ? 'box-shadow: 0 0 0 2px var(--sempa-bg-panel), 0 0 0 4px var(--sempa-accent);'
                               : ''}">
              </button>
            {/each}
          </div>
          <p class="mt-3 text-[10px]" style="color: var(--sempa-text-dim);">
            Currently: <span class="font-medium" style="color: var(--sempa-text-soft);">{ACCENT_PRESETS[theme.accent].label}</span>
          </p>
        </div>

        <!-- Text size -->
        <div>
          <p class="mb-3 text-xs font-medium" style="color: var(--sempa-text-soft);">Text size</p>
          <div style="display:flex; align-items:center; gap:12px;">
            <span style="font-size:12px; color:var(--sempa-text-dim)">A</span>
            <input type="range" min="80" max="130" step="5"
                   value={theme.textScale}
                   oninput={(e) => theme.setScale(parseInt((e.target as HTMLInputElement).value, 10))}
                   class="h-1.5 appearance-none rounded-full cursor-pointer"
                   style="flex:1; background: var(--sempa-border); accent-color: var(--sempa-accent);" />
            <span style="font-size:18px; font-weight:600; color:var(--sempa-text)">A</span>
            <span style="font-family:monospace; font-size:12px; color:var(--sempa-text-dim); width:36px">{theme.textScale}%</span>
          </div>
          <button onclick={() => theme.setScale(100)}
                  class="mt-2" style="color: var(--sempa-text-dim); font-size:12px; background:none; border:none; cursor:pointer;">
            Reset to default
          </button>
        </div>

        <!-- Mode: segmented pill -->
        <div>
          <p class="mb-3 text-xs font-medium" style="color: var(--sempa-text-soft);">Mode</p>
          <div style="display:flex; border-radius:9999px; border:1px solid var(--sempa-border);
                      padding:3px; gap:2px; width:fit-content;">
            <button onclick={() => { if (theme.dark) theme.toggle(); }}
                    class="transition-colors"
                    style="border-radius:9999px; padding:6px 16px; font-size:13px; border:none; cursor:pointer;
                           {!theme.dark
                             ? 'background: var(--sempa-accent-bg); color: var(--sempa-accent); font-weight:600;'
                             : 'background: transparent; color: var(--sempa-text-soft);'}">
              Light
            </button>
            <button onclick={() => { if (!theme.dark) theme.toggle(); }}
                    class="transition-colors"
                    style="border-radius:9999px; padding:6px 16px; font-size:13px; border:none; cursor:pointer;
                           {theme.dark
                             ? 'background: var(--sempa-accent-bg); color: var(--sempa-accent); font-weight:600;'
                             : 'background: transparent; color: var(--sempa-text-soft);'}">
              Dark
            </button>
          </div>
        </div>

        <!-- Contextual reflections toggle -->
        <div style="border-top: 1px solid var(--sempa-border); padding-top: 20px;">
          <div class="flex items-center justify-between gap-4">
            <div class="min-w-0">
              <p class="text-xs font-medium" style="color: var(--sempa-text-soft);">Show reflections in context</p>
              <p class="mt-1 text-[11px] leading-relaxed" style="color: var(--sempa-text-dim);">
                Surface your daily intention &amp; reflection on the day view, and the week-review
                summary on the week view. Turn off to keep them only in the Journal.
              </p>
            </div>
            <button onclick={prefs.toggleContextualReflections}
                    role="switch" aria-checked={prefs.contextualReflections}
                    aria-label="Show reflections in context"
                    class="relative shrink-0 rounded-full transition-colors"
                    style="width:44px; height:24px; border:none; cursor:pointer;
                           background: {prefs.contextualReflections ? 'var(--sempa-accent)' : 'var(--sempa-border)'};">
              <span class="absolute top-1/2 -translate-y-1/2 rounded-full bg-white transition-transform"
                    style="width:16px; height:16px; transform: translateX({prefs.contextualReflections ? '24px' : '4px'});"></span>
            </button>
          </div>
        </div>
      </div>
    </section>
  </div>
{/snippet}

<style>
  :root {
    --sempa-cal-feed-bg: #e8f4ee;
  }
  :global(.dark) {
    --sempa-cal-feed-bg: #1a2820;
  }
</style>
