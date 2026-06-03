<script lang="ts">
  import { onMount, tick } from 'svelte';
  import { page } from '$app/stores';
  import { api } from '$lib/api';
  import { theme, ACCENT_PRESETS, type AccentName } from '$lib/stores/theme.svelte';
  import type { ICalSubscription } from '$lib/types';

  type AccountStatus = { connected: boolean; email?: string; last_synced_at?: string | null; enabled?: boolean };

  let gmail    = $state<AccountStatus>({ connected: false });
  let calendar = $state<{ connected: boolean; email?: string; last_synced_at?: string }>({ connected: false });
  let fastmail = $state<AccountStatus>({ connected: false });
  let fmCal    = $state<{ connected: boolean; enabled: boolean; last_synced_at?: string | null }>({ connected: false, enabled: false });
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

  onMount(async () => {
    const connected = $page.url.searchParams.get('connected');
    if (connected === '1') window.history.replaceState({}, '', '/settings/accounts');

    [gmail, calendar, fastmail, fmCal, taskInbox, icalSubs] = await Promise.all([
      api.integrations.gmail.get(),
      api.integrations.calendar.get(),
      api.integrations.fastmail.get(),
      api.integrations.fastmail.calendar.get().catch(() => ({ connected: false, enabled: false })),
      api.integrations.taskInbox.get(),
      api.ical.listSubscriptions(),
    ]);

    if (fastmail.connected) {
      const jiraCfg = await api.integrations.jira.get().catch(() => ({ connected: false }));
      jira = jiraCfg;
    } else {
      jira = await api.integrations.jira.get().catch(() => ({ connected: false }));
    }
  });

  async function addIcalSub() {
    if (!icalUrl.trim()) return;
    icalAdding = true; icalError = '';
    try {
      const sub = await api.ical.createSubscription({
        name: icalName.trim() || new URL(icalUrl).hostname,
        url:  icalUrl.trim(),
        color: icalColor,
      });
      icalSubs = [...icalSubs, sub];
      icalUrl = ''; icalName = ''; showIcalForm = false;
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

  async function disconnectGmail() {
    if (!confirm('Disconnect Gmail? Imported tasks will be kept.')) return;
    await api.integrations.gmail.delete();
    gmail = { connected: false }; calendar = { connected: false };
  }

  async function disconnectFastmail() {
    if (!confirm('Disconnect Fastmail? Imported tasks will be kept.')) return;
    await api.integrations.fastmail.delete();
    fastmail = { connected: false }; fmCal = { connected: false, enabled: false };
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

  function sectionLabel(text: string) { return text; }
</script>

<div class="mx-auto max-w-xl px-6 py-8 pb-16">
  <h1 class="mb-1 text-xl font-semibold" style="color: var(--sempa-text);">Settings</h1>
  <p class="mb-8 text-sm" style="color: var(--sempa-text-soft);">Manage integrations, appearance, and tasks.</p>

  <!-- ════════════════════════════════════════════════════════════════════════
       SECTION: Email & Calendar
  ══════════════════════════════════════════════════════════════════════════ -->
  <p class="mb-3 text-xs font-semibold uppercase tracking-wider" style="color: var(--sempa-text-dim);">Email & Calendar</p>

  <!-- ── Gmail ─────────────────────────────────────────────────────────── -->
  <section class="mb-3 overflow-hidden rounded-xl border" style="border-color: var(--sempa-border); background: var(--sempa-bg-panel);">
    <div class="flex items-center gap-3 border-b px-5 py-4" style="border-color: var(--sempa-border);">
      <div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-red-50 dark:bg-red-950">
        <svg class="h-4 w-4 text-red-500" viewBox="0 0 24 24" fill="currentColor">
          <path d="M20 4H4c-1.1 0-2 .9-2 2v12c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V6c0-1.1-.9-2-2-2zm0 4-8 5-8-5V6l8 5 8-5v2z"/>
        </svg>
      </div>
      <div class="flex-1 min-w-0">
        <p class="text-sm font-semibold" style="color: var(--sempa-text);">Gmail</p>
        {#if gmail.connected}
          <p class="text-xs truncate" style="color: var(--sempa-text-soft);">{gmail.email}</p>
        {:else}
          <p class="text-xs" style="color: var(--sempa-text-dim);">Not connected</p>
        {/if}
      </div>
      {#if gmail.connected}
        <span class="inline-flex items-center gap-1 rounded-full bg-green-50 px-2 py-0.5 text-xs text-green-700 dark:bg-green-950 dark:text-green-400">
          <span class="h-1.5 w-1.5 rounded-full bg-green-500"></span>Connected
        </span>
      {/if}
    </div>

    {#if gmail.connected}
      <div class="px-5 py-4 space-y-3">
        <div class="flex items-center justify-between">
          <span class="text-xs" style="color: var(--sempa-text-soft);">Last synced: {formatDate(gmail.last_synced_at)}</span>
          <button onclick={() => syncService('gmail', api.integrations.gmail.sync)}
                  disabled={syncing['gmail']}
                  class="rounded-lg border px-3 py-1.5 text-xs font-medium transition-colors disabled:opacity-50"
                  style="border-color: var(--sempa-border); color: var(--sempa-text-soft);">
            {syncing['gmail'] ? 'Syncing…' : 'Sync starred'}
          </button>
        </div>
        {#if syncResults['gmail']}
          <p class="text-xs" style="color: var(--sempa-accent);">{syncResults['gmail']}</p>
        {/if}

        <!-- Google Calendar toggle -->
        <div class="flex items-center justify-between rounded-lg px-3 py-2.5" style="background: var(--sempa-accent-bg);">
          <div>
            <p class="text-sm font-medium" style="color: var(--sempa-text);">Google Calendar</p>
            <p class="text-xs" style="color: var(--sempa-text-dim);">Import today's events as tasks</p>
          </div>
          {#if calendar.connected}
            <div class="flex items-center gap-2">
              <button onclick={() => syncService('calendar', () => api.integrations.calendar.sync())}
                      disabled={syncing['calendar']}
                      class="rounded border px-2 py-1 text-xs transition-colors disabled:opacity-50"
                      style="border-color: var(--sempa-border); color: var(--sempa-text-soft);">
                {syncing['calendar'] ? 'Syncing…' : 'Sync today'}
              </button>
              <button onclick={() => toggleCalendar(false)} class="text-xs" style="color: var(--sempa-text-dim);">Disable</button>
            </div>
          {:else}
            <a href={api.integrations.gmail.authUrl(true)}
               class="rounded-lg px-3 py-1.5 text-xs font-medium text-white"
               style="background: var(--sempa-accent);">
              Connect Calendar
            </a>
          {/if}
        </div>
        {#if syncResults['calendar']}
          <p class="text-xs" style="color: var(--sempa-accent);">{syncResults['calendar']}</p>
        {/if}

        <button onclick={disconnectGmail} class="text-xs text-red-500 hover:text-red-700 dark:text-red-400">
          Disconnect Gmail
        </button>
      </div>
    {:else}
      <div class="px-5 py-5 text-center">
        <p class="mb-3 text-sm" style="color: var(--sempa-text-soft);">Import starred emails as tasks. Read-only access.</p>
        <a href={api.integrations.gmail.authUrl(false)}
           class="inline-flex items-center gap-2 rounded-lg border px-4 py-2 text-sm font-medium shadow-sm transition-colors"
           style="border-color: var(--sempa-border); background: var(--sempa-bg-panel); color: var(--sempa-text);">
          <svg class="h-4 w-4 text-red-500" viewBox="0 0 24 24" fill="currentColor">
            <path d="M20 4H4c-1.1 0-2 .9-2 2v12c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V6c0-1.1-.9-2-2-2zm0 4-8 5-8-5V6l8 5 8-5v2z"/>
          </svg>
          Connect with Google
        </a>
      </div>
    {/if}
  </section>

  <!-- ── Fastmail ────────────────────────────────────────────────────────── -->
  <section class="mb-3 overflow-hidden rounded-xl border" style="border-color: var(--sempa-border); background: var(--sempa-bg-panel);">
    <div class="flex items-center gap-3 border-b px-5 py-4" style="border-color: var(--sempa-border);">
      <div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg" style="background: var(--sempa-accent-bg);">
        <svg class="h-4 w-4" style="color: var(--sempa-accent);" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24">
          <path stroke-linecap="round" d="M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"/>
        </svg>
      </div>
      <div class="flex-1 min-w-0">
        <p class="text-sm font-semibold" style="color: var(--sempa-text);">Fastmail</p>
        {#if fastmail.connected}
          <p class="text-xs truncate" style="color: var(--sempa-text-soft);">{fastmail.email}</p>
        {:else}
          <p class="text-xs" style="color: var(--sempa-text-dim);">Not connected</p>
        {/if}
      </div>
      {#if fastmail.connected}
        <span class="inline-flex items-center gap-1 rounded-full bg-green-50 px-2 py-0.5 text-xs text-green-700 dark:bg-green-950 dark:text-green-400">
          <span class="h-1.5 w-1.5 rounded-full bg-green-500"></span>Connected
        </span>
      {/if}
    </div>

    {#if fastmail.connected}
      <div class="px-5 py-4 space-y-3">
        <div class="flex items-center justify-between">
          <span class="text-xs" style="color: var(--sempa-text-soft);">Last synced: {formatDate(fastmail.last_synced_at)}</span>
          <button onclick={() => syncService('fastmail', api.integrations.fastmail.sync)}
                  disabled={syncing['fastmail']}
                  class="rounded-lg border px-3 py-1.5 text-xs font-medium transition-colors disabled:opacity-50"
                  style="border-color: var(--sempa-border); color: var(--sempa-text-soft);">
            {syncing['fastmail'] ? 'Syncing…' : 'Sync starred'}
          </button>
        </div>
        {#if syncResults['fastmail']}
          <p class="text-xs" style="color: var(--sempa-accent);">{syncResults['fastmail']}</p>
        {/if}

        <!-- Fastmail Calendar toggle -->
        {#if fmCal.connected}
          <div class="space-y-2 rounded-lg px-3 py-2.5" style="background: var(--sempa-accent-bg);">
            <div class="flex items-center justify-between">
              <div>
                <p class="text-sm font-medium" style="color: var(--sempa-text);">Fastmail Calendar</p>
                <p class="text-xs" style="color: var(--sempa-text-dim);">Sync events via JMAP Calendars</p>
              </div>
              <!-- Toggle switch -->
              <button onclick={() => toggleFastmailCalendar(!fmCal.enabled)}
                      disabled={syncing['fmcal-toggle']}
                      class="relative inline-flex h-5 w-9 items-center rounded-full transition-colors disabled:opacity-50"
                      style="background: {fmCal.enabled ? 'var(--sempa-accent)' : 'var(--sempa-border)'};">
                <span class="inline-block h-3.5 w-3.5 transform rounded-full bg-white shadow transition-transform"
                      style="transform: translateX({fmCal.enabled ? '18px' : '3px'});"></span>
              </button>
            </div>
            {#if fmCal.enabled}
              <div class="flex items-center justify-between pt-1">
                <span class="text-xs" style="color: var(--sempa-text-soft);">Last synced: {formatDate(fmCal.last_synced_at)}</span>
                <button onclick={syncFastmailCalendar}
                        disabled={syncing['fmcal']}
                        class="rounded border px-2 py-1 text-xs transition-colors disabled:opacity-50"
                        style="border-color: var(--sempa-border); color: var(--sempa-text-soft);">
                  {syncing['fmcal'] ? 'Syncing…' : 'Sync now'}
                </button>
              </div>
              {#if syncResults['fmcal']}
                <p class="text-xs" style="color: var(--sempa-accent);">{syncResults['fmcal']}</p>
              {/if}
              <p class="text-[10px]" style="color: var(--sempa-text-dim);">
                Syncs 4 weeks of events into your schedule panel. Events flow into Sempa; full bidirectional write coming soon.
              </p>
            {/if}
          </div>
        {/if}

        <button onclick={disconnectFastmail} class="text-xs text-red-500 hover:text-red-700 dark:text-red-400">
          Disconnect Fastmail
        </button>
      </div>
    {:else}
      <div class="px-5 py-5">
        {#if !fmShowForm}
          <p class="mb-3 text-sm" style="color: var(--sempa-text-soft);">Import starred emails and sync your calendar using a Fastmail app password.</p>
          <button onclick={() => fmShowForm = true}
                  class="rounded-lg px-4 py-2 text-sm font-medium text-white"
                  style="background: var(--sempa-accent);">
            Connect Fastmail
          </button>
        {:else}
          <div class="space-y-3">
            <div>
              <label class="mb-1 block text-xs font-medium" style="color: var(--sempa-text-soft);" for="fm-email">Email</label>
              <input id="fm-email" type="email" bind:value={fmEmail} placeholder="you@fastmail.com"
                     class="w-full rounded-lg border px-3 py-2 text-sm outline-none"
                     style="border-color: var(--sempa-border); background: var(--sempa-bg-main); color: var(--sempa-text);" />
            </div>
            <div>
              <label class="mb-1 block text-xs font-medium" style="color: var(--sempa-text-soft);" for="fm-pass">App Password</label>
              <input id="fm-pass" type="password" bind:value={fmPassword}
                     placeholder="Generate at fastmail.com → Settings → Security"
                     class="w-full rounded-lg border px-3 py-2 text-sm outline-none"
                     style="border-color: var(--sempa-border); background: var(--sempa-bg-main); color: var(--sempa-text);" />
              <p class="mt-1 text-xs" style="color: var(--sempa-text-dim);">
                Create at fastmail.com → Settings → Privacy & Security → App Passwords
              </p>
            </div>
            {#if fmError}<p class="text-sm text-red-600 dark:text-red-400">{fmError}</p>{/if}
            <div class="flex gap-2">
              <button onclick={connectFastmail} disabled={fmSaving || !fmEmail || !fmPassword}
                      class="rounded-lg px-4 py-2 text-sm font-medium text-white disabled:opacity-40"
                      style="background: var(--sempa-accent);">
                {fmSaving ? 'Connecting…' : 'Connect'}
              </button>
              <button onclick={() => { fmShowForm = false; fmError = ''; }}
                      class="rounded-lg border px-4 py-2 text-sm transition-colors"
                      style="border-color: var(--sempa-border); color: var(--sempa-text-soft);">
                Cancel
              </button>
            </div>
          </div>
        {/if}
      </div>
    {/if}
  </section>

  <!-- ── Calendar Feeds (ICS) ──────────────────────────────────────────── -->
  <section class="mb-3 overflow-hidden rounded-xl border" style="border-color: var(--sempa-border); background: var(--sempa-bg-panel);">
    <div class="flex items-center justify-between border-b px-5 py-4" style="border-color: var(--sempa-border);">
      <div>
        <h2 class="text-sm font-semibold" style="color: var(--sempa-text);">Calendar Feeds</h2>
        <p class="mt-0.5 text-xs" style="color: var(--sempa-text-dim);">Subscribe to any ICS/webcal URL for read-only events in the Schedule panel</p>
      </div>
      <button onclick={openIcalForm}
              class="rounded-lg border px-3 py-1.5 text-xs font-medium transition-colors"
              style="border-color: var(--sempa-border); color: var(--sempa-text-soft);">
        + Add feed
      </button>
    </div>

    <div class="px-5 py-4 space-y-3">
      {#if icalSubs.length === 0 && !showIcalForm}
        <p class="text-sm" style="color: var(--sempa-text-dim);">
          No calendar feeds yet. Add a webcal or ICS URL — useful for work calendars, public holidays, etc.
        </p>
      {/if}

      {#each icalSubs as sub (sub.id)}
        <div class="flex items-center gap-3 rounded-lg border px-3 py-2.5" style="border-color: var(--sempa-border);">
          <div class="h-3 w-3 shrink-0 rounded-full" style="background:{sub.color}"></div>
          <div class="flex-1 min-w-0">
            <p class="text-sm font-medium truncate" style="color: var(--sempa-text);">{sub.name}</p>
            <p class="text-xs truncate" style="color: var(--sempa-text-dim);">{sub.url}</p>
            {#if sub.error_msg}
              <p class="text-xs text-red-500 dark:text-red-400">Error: {sub.error_msg}</p>
            {:else if sub.last_synced_at}
              <p class="text-xs" style="color: var(--sempa-text-dim);">Last synced: {new Date(sub.last_synced_at).toLocaleString()}</p>
            {/if}
          </div>
          <button onclick={() => syncIcalSub(sub.id)} disabled={syncing['ical_' + sub.id]}
                  class="text-xs disabled:opacity-40 transition-colors"
                  style="color: var(--sempa-text-dim);">
            {syncing['ical_' + sub.id] ? '…' : 'Sync'}
          </button>
          <button onclick={() => removeIcalSub(sub.id)} aria-label="Remove feed"
                  class="text-gray-300 hover:text-red-400 transition-colors dark:text-gray-600">
            <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
              <path stroke-linecap="round" d="M6 18L18 6M6 6l12 12"/>
            </svg>
          </button>
        </div>
      {/each}

      {#if showIcalForm}
        <div bind:this={icalFormEl}
             class="space-y-3 rounded-xl border p-4" style="border-color: var(--sempa-border); background: var(--sempa-bg-main);">
          <div>
            <label class="mb-1 block text-xs font-medium" style="color: var(--sempa-text-soft);" for="ical-url">
              ICS / Webcal URL <span class="text-red-400">*</span>
            </label>
            <input id="ical-url" type="url" bind:value={icalUrl}
                   placeholder="https://example.com/calendar.ics  or  webcal://..."
                   class="w-full rounded-lg border px-3 py-2 text-sm outline-none"
                   style="border-color: var(--sempa-border); background: var(--sempa-bg-panel); color: var(--sempa-text);" />
            <p class="mt-1 text-[10px]" style="color: var(--sempa-text-dim);">
              Paste the ICS link — works with Google Calendar, Fastmail, Outlook, etc.
            </p>
          </div>
          <div class="grid grid-cols-2 gap-3">
            <div>
              <label class="mb-1 block text-xs font-medium" style="color: var(--sempa-text-soft);" for="ical-name">Name (optional)</label>
              <input id="ical-name" type="text" bind:value={icalName}
                     placeholder="Work calendar"
                     class="w-full rounded-lg border px-3 py-2 text-sm outline-none"
                     style="border-color: var(--sempa-border); background: var(--sempa-bg-panel); color: var(--sempa-text);" />
            </div>
            <div>
              <label class="mb-1 block text-xs font-medium" style="color: var(--sempa-text-soft);" for="ical-color">Colour</label>
              <div class="flex items-center gap-2">
                <input id="ical-color" type="color" bind:value={icalColor}
                       class="h-9 w-14 cursor-pointer rounded-lg border p-1"
                       style="border-color: var(--sempa-border); background: var(--sempa-bg-panel);" />
                <span class="text-xs font-mono" style="color: var(--sempa-text-dim);">{icalColor}</span>
              </div>
            </div>
          </div>
          {#if icalError}<p class="text-sm text-red-600 dark:text-red-400">{icalError}</p>{/if}
          <div class="flex gap-2">
            <button onclick={addIcalSub} disabled={icalAdding || !icalUrl.trim()}
                    class="rounded-lg px-4 py-2 text-sm font-medium text-white disabled:opacity-40 transition-colors"
                    style="background: var(--sempa-accent);">
              {icalAdding ? 'Adding…' : 'Subscribe'}
            </button>
            <button onclick={() => { showIcalForm = false; icalError = ''; }}
                    class="rounded-lg border px-4 py-2 text-sm transition-colors"
                    style="border-color: var(--sempa-border); color: var(--sempa-text-soft);">
              Cancel
            </button>
          </div>
        </div>
      {/if}
    </div>
  </section>

  <!-- ── Email Inbox (task forwarding) ─────────────────────────────────── -->
  <section class="mb-8 overflow-hidden rounded-xl border" style="border-color: var(--sempa-border); background: var(--sempa-bg-panel);">
    <div class="flex items-center gap-3 border-b px-5 py-4" style="border-color: var(--sempa-border);">
      <div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-violet-50 dark:bg-violet-950">
        <svg class="h-4 w-4 text-violet-500" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" d="M3 10l9-7 9 7v8a2 2 0 01-2 2H5a2 2 0 01-2-2v-8z"/>
          <path stroke-linecap="round" stroke-linejoin="round" d="M9 21V12h6v9"/>
        </svg>
      </div>
      <div class="flex-1 min-w-0">
        <p class="text-sm font-semibold" style="color: var(--sempa-text);">Email Inbox</p>
        {#if taskInbox.connected}
          <p class="text-xs font-mono truncate" style="color: var(--sempa-text-soft);">{taskInbox.inbox_address}</p>
        {:else}
          <p class="text-xs" style="color: var(--sempa-text-dim);">Forward emails here to create tasks</p>
        {/if}
      </div>
      {#if taskInbox.connected}
        <span class="inline-flex items-center gap-1 rounded-full bg-green-50 px-2 py-0.5 text-xs text-green-700 dark:bg-green-950 dark:text-green-400">
          <span class="h-1.5 w-1.5 rounded-full bg-green-500"></span>Active
        </span>
      {/if}
    </div>

    {#if taskInbox.connected}
      <div class="px-5 py-4 space-y-4">
        <div class="flex items-center justify-between">
          <span class="text-xs" style="color: var(--sempa-text-soft);">Last synced: {formatDate(taskInbox.last_synced_at)}</span>
          <button onclick={() => syncService('task-inbox', api.integrations.taskInbox.sync)}
                  disabled={syncing['task-inbox']}
                  class="rounded-lg border px-3 py-1.5 text-xs font-medium transition-colors disabled:opacity-50"
                  style="border-color: var(--sempa-border); color: var(--sempa-text-soft);">
            {syncing['task-inbox'] ? 'Syncing…' : 'Sync now'}
          </button>
        </div>
        {#if syncResults['task-inbox']}
          <p class="text-xs" style="color: var(--sempa-accent);">{syncResults['task-inbox']}</p>
        {/if}

        <!-- Allowed senders -->
        <div class="space-y-2">
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
                  <button onclick={() => removeSender(sender)} class="hover:text-red-500 transition-colors">
                    <svg class="h-3 w-3" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                      <path stroke-linecap="round" d="M6 18L18 6M6 6l12 12"/>
                    </svg>
                  </button>
                </span>
              {/each}
            </div>
          {:else}
            <p class="text-xs italic" style="color: var(--sempa-text-dim);">
              No restrictions — add domains or addresses below to limit who can create tasks.
            </p>
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

        <button onclick={disconnectTaskInbox} class="text-xs text-red-500 hover:text-red-700 dark:text-red-400">
          Remove email inbox
        </button>
      </div>
    {:else}
      <div class="px-5 py-5">
        {#if !tiShowForm}
          <p class="mb-3 text-sm" style="color: var(--sempa-text-soft);">
            Forward any email to a Fastmail address and Sempa will create a task from it.
          </p>
          <button onclick={() => tiShowForm = true}
                  class="rounded-lg bg-violet-500 px-4 py-2 text-sm font-medium text-white hover:bg-violet-600">
            Set up email inbox
          </button>
        {:else}
          <div class="space-y-3">
            <div>
              <label class="mb-1 block text-xs font-medium" style="color: var(--sempa-text-soft);" for="ti-email">Fastmail email</label>
              <input id="ti-email" type="email" bind:value={tiEmail} placeholder="you@fastmail.com"
                     class="w-full rounded-lg border px-3 py-2 text-sm outline-none"
                     style="border-color: var(--sempa-border); background: var(--sempa-bg-main); color: var(--sempa-text);" />
            </div>
            <div>
              <label class="mb-1 block text-xs font-medium" style="color: var(--sempa-text-soft);" for="ti-pass">App password</label>
              <input id="ti-pass" type="password" bind:value={tiPassword}
                     placeholder="Generate at Fastmail → Settings → Privacy & Security"
                     class="w-full rounded-lg border px-3 py-2 text-sm outline-none"
                     style="border-color: var(--sempa-border); background: var(--sempa-bg-main); color: var(--sempa-text);" />
            </div>
            <div>
              <label class="mb-1 block text-xs font-medium" style="color: var(--sempa-text-soft);" for="ti-addr">Forwarding address</label>
              <input id="ti-addr" type="email" bind:value={tiAddress} placeholder="tasks@sempa.ca"
                     class="w-full rounded-lg border px-3 py-2 text-sm outline-none"
                     style="border-color: var(--sempa-border); background: var(--sempa-bg-main); color: var(--sempa-text);" />
            </div>
            {#if tiError}<p class="text-sm text-red-600 dark:text-red-400">{tiError}</p>{/if}
            <div class="flex gap-2">
              <button onclick={connectTaskInbox} disabled={tiSaving || !tiEmail || !tiPassword || !tiAddress}
                      class="rounded-lg bg-violet-500 px-4 py-2 text-sm font-medium text-white hover:bg-violet-600 disabled:opacity-40">
                {tiSaving ? 'Connecting…' : 'Connect'}
              </button>
              <button onclick={() => { tiShowForm = false; tiError = ''; }}
                      class="rounded-lg border px-4 py-2 text-sm transition-colors"
                      style="border-color: var(--sempa-border); color: var(--sempa-text-soft);">
                Cancel
              </button>
            </div>
          </div>
        {/if}
      </div>
    {/if}
  </section>

  <!-- ════════════════════════════════════════════════════════════════════════
       SECTION: Project Management
  ══════════════════════════════════════════════════════════════════════════ -->
  <p class="mb-3 text-xs font-semibold uppercase tracking-wider" style="color: var(--sempa-text-dim);">Project Management</p>

  <div class="mb-8 flex flex-col gap-2">
    <a href="/settings/integrations/jira"
       class="flex items-center gap-4 rounded-xl border px-5 py-4 transition-colors"
       style="border-color: var(--sempa-border); background: var(--sempa-bg-panel);">
      <div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg" style="background: var(--sempa-accent-bg);">
        <svg class="h-5 w-5" style="color: var(--sempa-accent);" viewBox="0 0 24 24" fill="currentColor">
          <path d="M11.571 11.513H0a5.218 5.218 0 0 0 5.232 5.215h2.13v2.057A5.215 5.215 0 0 0 12.575 24V12.518a1.005 1.005 0 0 0-1.005-1.005zm5.723-5.756H5.757a5.215 5.215 0 0 0 5.214 5.214h2.129v2.058A5.218 5.218 0 0 0 18.313 18.3V6.763a1.006 1.006 0 0 0-1.019-1.006zM23.277.007H11.749a5.215 5.215 0 0 0 5.214 5.214h2.129v2.058A5.218 5.218 0 0 0 24.282 12.5V1.012A1.005 1.005 0 0 0 23.277.007z"/>
        </svg>
      </div>
      <div class="flex-1 min-w-0">
        <p class="font-medium" style="color: var(--sempa-text);">Jira</p>
        {#if jira.connected}
          <p class="text-sm" style="color: var(--sempa-text-soft);">Connected — syncs assigned issues</p>
        {:else}
          <p class="text-sm" style="color: var(--sempa-text-dim);">Not connected</p>
        {/if}
      </div>
      <div class="flex items-center gap-2">
        {#if jira.connected}
          <span class="inline-flex items-center gap-1 rounded-full bg-green-50 px-2.5 py-0.5 text-xs font-medium text-green-700 dark:bg-green-950 dark:text-green-400">
            <span class="h-1.5 w-1.5 rounded-full bg-green-500"></span>Connected
          </span>
        {/if}
        <svg class="h-4 w-4" style="color: var(--sempa-text-dim);" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
          <path stroke-linecap="round" d="m9 18 6-6-6-6"/>
        </svg>
      </div>
    </a>
  </div>

  <!-- ════════════════════════════════════════════════════════════════════════
       SECTION: Tasks
  ══════════════════════════════════════════════════════════════════════════ -->
  <p class="mb-3 text-xs font-semibold uppercase tracking-wider" style="color: var(--sempa-text-dim);">Tasks</p>

  <div class="mb-8 flex flex-col gap-2">
    <a href="/settings/tags"
       class="flex items-center gap-4 rounded-xl border px-5 py-4 transition-colors"
       style="border-color: var(--sempa-border); background: var(--sempa-bg-panel);">
      <div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-violet-50 dark:bg-violet-950">
        <svg class="h-5 w-5 text-violet-500" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24">
          <path stroke-linecap="round" d="M7 7h.01M7 3h5c.512 0 1.024.195 1.414.586l7 7a2 2 0 0 1 0 2.828l-7 7a2 2 0 0 1-2.828 0l-7-7A2 2 0 0 1 3 12V7a4 4 0 0 1 4-4z"/>
        </svg>
      </div>
      <div class="flex-1">
        <p class="font-medium" style="color: var(--sempa-text);">Tags</p>
        <p class="text-sm" style="color: var(--sempa-text-soft);">Colour-coded labels for your tasks</p>
      </div>
      <svg class="h-4 w-4" style="color: var(--sempa-text-dim);" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
        <path stroke-linecap="round" d="m9 18 6-6-6-6"/>
      </svg>
    </a>

    <a href="/settings/recurring"
       class="flex items-center gap-4 rounded-xl border px-5 py-4 transition-colors"
       style="border-color: var(--sempa-border); background: var(--sempa-bg-panel);">
      <div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg" style="background: var(--sempa-accent-bg);">
        <span class="text-xl" style="color: var(--sempa-accent);">↺</span>
      </div>
      <div class="flex-1">
        <p class="font-medium" style="color: var(--sempa-text);">Recurring Tasks</p>
        <p class="text-sm" style="color: var(--sempa-text-soft);">Daily, weekly, and monthly templates</p>
      </div>
      <svg class="h-4 w-4" style="color: var(--sempa-text-dim);" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
        <path stroke-linecap="round" d="m9 18 6-6-6-6"/>
      </svg>
    </a>
  </div>

  <!-- ════════════════════════════════════════════════════════════════════════
       SECTION: Appearance
  ══════════════════════════════════════════════════════════════════════════ -->
  <p class="mb-3 text-xs font-semibold uppercase tracking-wider" style="color: var(--sempa-text-dim);">Appearance</p>

  <section class="overflow-hidden rounded-xl border" style="border-color: var(--sempa-border); background: var(--sempa-bg-panel);">
    <div class="px-5 py-4 space-y-5">
      <!-- Accent colour -->
      <div>
        <p class="mb-3 text-xs font-medium" style="color: var(--sempa-text-soft);">Accent colour</p>
        <div class="flex flex-wrap gap-2">
          {#each Object.entries(ACCENT_PRESETS) as [name, preset]}
            <button onclick={() => theme.setAccent(name as AccentName)}
                    title={preset.label}
                    class="group relative flex h-8 w-8 items-center justify-center rounded-full
                           border-2 transition-all hover:scale-110
                           {theme.accent === name
                             ? 'border-gray-500 scale-110 shadow-md dark:border-gray-400'
                             : 'border-transparent hover:border-gray-300 dark:hover:border-gray-500'}"
                    style="background:{preset.swatch}">
              {#if theme.accent === name}
                <svg class="h-4 w-4 text-white drop-shadow" fill="none" stroke="currentColor" stroke-width="3" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/>
                </svg>
              {/if}
              <span class="pointer-events-none absolute -bottom-6 left-1/2 -translate-x-1/2 whitespace-nowrap
                           rounded bg-gray-800 px-1.5 py-0.5 text-[10px] text-white opacity-0
                           group-hover:opacity-100 transition-opacity dark:bg-gray-600">
                {preset.label}
              </span>
            </button>
          {/each}
        </div>
        <p class="mt-3 text-[10px]" style="color: var(--sempa-text-dim);">
          Currently: <span class="font-medium" style="color: var(--sempa-text-soft);">{ACCENT_PRESETS[theme.accent].label}</span>
        </p>
      </div>

      <!-- Text size -->
      <div>
        <div class="mb-3 flex items-center justify-between">
          <p class="text-xs font-medium" style="color: var(--sempa-text-soft);">Text size</p>
          <span class="tabular-nums text-xs" style="color: var(--sempa-text-dim);">{theme.textScale}%</span>
        </div>
        <div class="flex items-center gap-3">
          <span class="text-xs" style="color: var(--sempa-text-dim);">A</span>
          <input type="range" min="80" max="130" step="5"
                 value={theme.textScale}
                 oninput={(e) => theme.setScale(parseInt((e.target as HTMLInputElement).value, 10))}
                 class="flex-1 h-1.5 appearance-none rounded-full cursor-pointer"
                 style="background: var(--sempa-border); accent-color: var(--sempa-accent);" />
          <span class="text-base" style="color: var(--sempa-text-dim);">A</span>
        </div>
        <button onclick={() => theme.setScale(100)}
                class="mt-2 text-xs underline" style="color: var(--sempa-text-dim);">
          Reset to default
        </button>
      </div>

      <!-- Dark / light -->
      <div>
        <p class="mb-3 text-xs font-medium" style="color: var(--sempa-text-soft);">Mode</p>
        <button onclick={theme.toggle}
                class="flex items-center gap-2 rounded-lg border px-4 py-2 text-sm transition-colors"
                style="border-color: var(--sempa-border); color: var(--sempa-text-soft);">
          {theme.dark ? '☀️ Switch to light mode' : '🌙 Switch to dark mode'}
        </button>
      </div>
    </div>
  </section>
</div>
