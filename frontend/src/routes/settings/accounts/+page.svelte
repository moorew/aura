<script lang="ts">
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import { api } from '$lib/api';

  type AccountStatus = {
    connected: boolean;
    email?: string;
    last_synced_at?: string | null;
    enabled?: boolean;
  };

  let gmail = $state<AccountStatus>({ connected: false });
  let calendar = $state<{ connected: boolean; email?: string; last_synced_at?: string }>({ connected: false });
  let fastmail = $state<AccountStatus>({ connected: false });
  let emailFwd = $state<{
    smtp_enabled: boolean; smtp_address: string; smtp_port: string;
    webhook_enabled: boolean; webhook_url: string;
  }>({ smtp_enabled: false, smtp_address: '', smtp_port: '', webhook_enabled: false, webhook_url: '' });
  let fwdCopied = $state(false);

  let fmEmail = $state('');
  let fmPassword = $state('');
  let fmSaving = $state(false);
  let fmError = $state('');
  let fmShowForm = $state(false);

  let syncing = $state<Record<string, boolean>>({});
  let syncResults = $state<Record<string, string>>({});

  onMount(async () => {
    // Check if returning from Gmail OAuth
    const connected = $page.url.searchParams.get('connected');
    if (connected === '1') {
      window.history.replaceState({}, '', '/settings/accounts');
    }

    [gmail, calendar, fastmail, emailFwd] = await Promise.all([
      api.integrations.gmail.get(),
      api.integrations.calendar.get(),
      api.integrations.fastmail.get(),
      api.integrations.emailForward.get(),
    ]);
  });

  async function syncService(name: string, fn: () => Promise<{ new: number; updated: number; errors: number }>) {
    syncing[name] = true;
    syncResults[name] = '';
    try {
      const r = await fn();
      syncResults[name] = `${r.new} new, ${r.updated} updated${r.errors ? `, ${r.errors} errors` : ''}`;
    } catch (e) {
      syncResults[name] = 'Error: ' + (e as Error).message;
    } finally {
      syncing[name] = false;
    }
  }

  async function connectFastmail() {
    if (!fmEmail.trim() || !fmPassword.trim()) return;
    fmSaving = true; fmError = '';
    try {
      await api.integrations.fastmail.save(fmEmail.trim(), fmPassword.trim());
      fastmail = await api.integrations.fastmail.get();
      fmShowForm = false; fmEmail = ''; fmPassword = '';
    } catch (e) {
      fmError = (e as Error).message;
    } finally {
      fmSaving = false;
    }
  }

  async function toggleCalendar(enabled: boolean) {
    await api.integrations.calendar.toggle(enabled);
    calendar = await api.integrations.calendar.get();
  }

  async function disconnectGmail() {
    if (!confirm('Disconnect Gmail? Imported tasks will be kept.')) return;
    await api.integrations.gmail.delete();
    gmail = { connected: false };
    calendar = { connected: false };
  }

  async function disconnectFastmail() {
    if (!confirm('Disconnect Fastmail? Imported tasks will be kept.')) return;
    await api.integrations.fastmail.delete();
    fastmail = { connected: false };
  }

  function formatDate(s?: string | null) {
    if (!s) return 'Never';
    return new Date(s).toLocaleString();
  }

  async function copyAddress() {
    await navigator.clipboard.writeText(emailFwd.address);
    fwdCopied = true;
    setTimeout(() => (fwdCopied = false), 2000);
  }
</script>

<div class="mx-auto max-w-xl px-6 py-8">
  <h1 class="mb-1 text-xl font-semibold text-gray-900 dark:text-gray-50">Accounts</h1>
  <p class="mb-8 text-sm text-gray-500 dark:text-gray-400">
    Connect email and calendar accounts to import tasks automatically.
  </p>

  <!-- ── Gmail ──────────────────────────────────────────────────────────── -->
  <section class="mb-5 rounded-xl border border-gray-200 bg-white dark:border-gray-700 dark:bg-gray-800">
    <div class="flex items-center gap-3 border-b border-gray-100 px-5 py-4 dark:border-gray-700">
      <div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-red-50 dark:bg-red-950">
        <svg class="h-4 w-4 text-red-500" viewBox="0 0 24 24" fill="currentColor">
          <path d="M20 4H4c-1.1 0-2 .9-2 2v12c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V6c0-1.1-.9-2-2-2zm0 4-8 5-8-5V6l8 5 8-5v2z"/>
        </svg>
      </div>
      <div class="flex-1 min-w-0">
        <p class="text-sm font-semibold text-gray-800 dark:text-gray-100">Gmail</p>
        {#if gmail.connected}
          <p class="text-xs text-gray-500 dark:text-gray-400 truncate">{gmail.email}</p>
        {:else}
          <p class="text-xs text-gray-400 dark:text-gray-600">Not connected</p>
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
          <span class="text-xs text-gray-500 dark:text-gray-400">Last synced: {formatDate(gmail.last_synced_at)}</span>
          <button onclick={() => syncService('gmail', api.integrations.gmail.sync)}
                  disabled={syncing['gmail']}
                  class="rounded-lg border border-gray-200 px-3 py-1.5 text-xs font-medium text-gray-700
                         hover:bg-gray-50 disabled:opacity-50 dark:border-gray-600 dark:text-gray-300 dark:hover:bg-gray-700">
            {syncing['gmail'] ? 'Syncing…' : 'Sync starred'}
          </button>
        </div>
        {#if syncResults['gmail']}
          <p class="text-xs text-blue-600 dark:text-blue-400">{syncResults['gmail']}</p>
        {/if}

        <!-- Calendar toggle -->
        <div class="flex items-center justify-between rounded-lg bg-gray-50 px-3 py-2.5 dark:bg-gray-700/50">
          <div>
            <p class="text-sm font-medium text-gray-700 dark:text-gray-200">Google Calendar</p>
            <p class="text-xs text-gray-400 dark:text-gray-500">Import today's events as tasks</p>
          </div>
          {#if calendar.connected}
            <div class="flex items-center gap-2">
              <button onclick={() => syncService('calendar', () => api.integrations.calendar.sync())}
                      disabled={syncing['calendar']}
                      class="rounded border border-gray-200 px-2 py-1 text-xs text-gray-600
                             hover:bg-gray-100 disabled:opacity-50 dark:border-gray-600 dark:text-gray-300 dark:hover:bg-gray-600">
                {syncing['calendar'] ? 'Syncing…' : 'Sync today'}
              </button>
              <button onclick={() => toggleCalendar(false)}
                      class="text-xs text-gray-400 hover:text-red-500 dark:text-gray-600 dark:hover:text-red-400">
                Disable
              </button>
            </div>
          {:else}
            <a href={api.integrations.gmail.authUrl(true)}
               class="rounded-lg bg-blue-500 px-3 py-1.5 text-xs font-medium text-white hover:bg-blue-600">
              Connect Calendar
            </a>
          {/if}
        </div>
        {#if syncResults['calendar']}
          <p class="text-xs text-blue-600 dark:text-blue-400">{syncResults['calendar']}</p>
        {/if}

        <button onclick={disconnectGmail}
                class="text-xs text-red-500 hover:text-red-700 dark:text-red-400 dark:hover:text-red-300">
          Disconnect Gmail
        </button>
      </div>
    {:else}
      <div class="px-5 py-5 text-center">
        <p class="mb-3 text-sm text-gray-500 dark:text-gray-400">Import starred emails as tasks. Read-only access.</p>
        <a href={api.integrations.gmail.authUrl(false)}
           class="inline-flex items-center gap-2 rounded-lg border border-gray-200 bg-white px-4 py-2
                  text-sm font-medium text-gray-700 shadow-sm hover:bg-gray-50
                  dark:border-gray-600 dark:bg-gray-700 dark:text-gray-200 dark:hover:bg-gray-600">
          <svg class="h-4 w-4 text-red-500" viewBox="0 0 24 24" fill="currentColor">
            <path d="M20 4H4c-1.1 0-2 .9-2 2v12c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V6c0-1.1-.9-2-2-2zm0 4-8 5-8-5V6l8 5 8-5v2z"/>
          </svg>
          Connect with Google
        </a>
      </div>
    {/if}
  </section>

  <!-- ── Email forwarding ──────────────────────────────────────────────── -->
  {#if emailFwd.webhook_enabled || emailFwd.smtp_enabled}
  <section class="mb-5 rounded-xl border border-gray-200 bg-white dark:border-gray-700 dark:bg-gray-800">
    <div class="flex items-center gap-3 border-b border-gray-100 px-5 py-4 dark:border-gray-700">
      <div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-violet-50 dark:bg-violet-950">
        <svg class="h-4 w-4 text-violet-500" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" d="M3 10l9-7 9 7v8a2 2 0 01-2 2H5a2 2 0 01-2-2v-8z"/>
          <path stroke-linecap="round" stroke-linejoin="round" d="M9 21V12h6v9"/>
        </svg>
      </div>
      <div class="flex-1 min-w-0">
        <p class="text-sm font-semibold text-gray-800 dark:text-gray-100">Email forwarding</p>
        <p class="text-xs text-gray-400 dark:text-gray-600">Forward any email to add it as a task</p>
      </div>
      <span class="inline-flex items-center gap-1 rounded-full bg-green-50 px-2 py-0.5 text-xs text-green-700 dark:bg-green-950 dark:text-green-400">
        <span class="h-1.5 w-1.5 rounded-full bg-green-500"></span>Active
      </span>
    </div>
    <div class="px-5 py-4 space-y-4">
      <p class="text-xs text-gray-500 dark:text-gray-400">
        Forward any email to Aura. The subject becomes the task title, body becomes notes. Tasks land in today's planned column.
      </p>

      {#if emailFwd.webhook_enabled}
        <div class="space-y-1.5">
          <p class="text-xs font-medium text-gray-600 dark:text-gray-400">Cloudflare Email Routing address</p>
          <div class="flex items-center gap-2 rounded-lg bg-gray-50 px-3 py-2.5 dark:bg-gray-700/50">
            <code class="flex-1 text-xs font-mono text-gray-700 dark:text-gray-200 break-all">tasks@sempa.ca</code>
            <button onclick={() => { navigator.clipboard.writeText('tasks@sempa.ca'); fwdCopied = true; setTimeout(() => fwdCopied = false, 2000); }}
                    class="shrink-0 rounded border border-gray-200 px-2 py-1 text-xs text-gray-500
                           hover:bg-gray-100 dark:border-gray-600 dark:text-gray-400 dark:hover:bg-gray-600 transition-colors">
              {fwdCopied ? 'Copied!' : 'Copy'}
            </button>
          </div>
          <p class="text-xs text-gray-400 dark:text-gray-600">Works from any email client, no port configuration needed.</p>
        </div>
      {/if}

      {#if emailFwd.smtp_enabled}
        <div class="space-y-1.5">
          <p class="text-xs font-medium text-gray-600 dark:text-gray-400">Direct SMTP (Tailscale-only)</p>
          <div class="flex items-center gap-2 rounded-lg bg-gray-50 px-3 py-2.5 dark:bg-gray-700/50">
            <code class="flex-1 text-xs font-mono text-gray-700 dark:text-gray-200 break-all">{emailFwd.smtp_address}</code>
            <button onclick={copyAddress}
                    class="shrink-0 rounded border border-gray-200 px-2 py-1 text-xs text-gray-500
                           hover:bg-gray-100 dark:border-gray-600 dark:text-gray-400 dark:hover:bg-gray-600 transition-colors">
              {fwdCopied ? 'Copied!' : 'Copy'}
            </button>
          </div>
          <p class="text-xs text-gray-400 dark:text-gray-600">Requires specifying port {emailFwd.smtp_port}. Use from desktop email clients on Tailscale.</p>
        </div>
      {/if}
    </div>
  </section>
  {/if}

  <!-- ── Fastmail ───────────────────────────────────────────────────────── -->
  <section class="mb-5 rounded-xl border border-gray-200 bg-white dark:border-gray-700 dark:bg-gray-800">
    <div class="flex items-center gap-3 border-b border-gray-100 px-5 py-4 dark:border-gray-700">
      <div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-blue-50 dark:bg-blue-950">
        <svg class="h-4 w-4 text-blue-500" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24">
          <path stroke-linecap="round" d="M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"/>
        </svg>
      </div>
      <div class="flex-1 min-w-0">
        <p class="text-sm font-semibold text-gray-800 dark:text-gray-100">Fastmail</p>
        {#if fastmail.connected}
          <p class="text-xs text-gray-500 dark:text-gray-400 truncate">{fastmail.email}</p>
        {:else}
          <p class="text-xs text-gray-400 dark:text-gray-600">Not connected</p>
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
          <span class="text-xs text-gray-500 dark:text-gray-400">Last synced: {formatDate(fastmail.last_synced_at)}</span>
          <button onclick={() => syncService('fastmail', api.integrations.fastmail.sync)}
                  disabled={syncing['fastmail']}
                  class="rounded-lg border border-gray-200 px-3 py-1.5 text-xs font-medium text-gray-700
                         hover:bg-gray-50 disabled:opacity-50 dark:border-gray-600 dark:text-gray-300 dark:hover:bg-gray-700">
            {syncing['fastmail'] ? 'Syncing…' : 'Sync flagged'}
          </button>
        </div>
        {#if syncResults['fastmail']}
          <p class="text-xs text-blue-600 dark:text-blue-400">{syncResults['fastmail']}</p>
        {/if}
        <button onclick={disconnectFastmail}
                class="text-xs text-red-500 hover:text-red-700 dark:text-red-400 dark:hover:text-red-300">
          Disconnect Fastmail
        </button>
      </div>
    {:else}
      <div class="px-5 py-5">
        {#if !fmShowForm}
          <p class="mb-3 text-sm text-gray-500 dark:text-gray-400">Import flagged emails using a Fastmail app password.</p>
          <button onclick={() => fmShowForm = true}
                  class="rounded-lg bg-blue-500 px-4 py-2 text-sm font-medium text-white hover:bg-blue-600">
            Connect Fastmail
          </button>
        {:else}
          <div class="space-y-3">
            <div>
              <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400" for="fm-email">Email</label>
              <input id="fm-email" type="email" bind:value={fmEmail} placeholder="you@fastmail.com"
                     class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm outline-none
                            focus:border-blue-500 focus:ring-2 focus:ring-blue-100
                            dark:border-gray-600 dark:bg-gray-700 dark:text-gray-100" />
            </div>
            <div>
              <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400" for="fm-pass">App Password</label>
              <input id="fm-pass" type="password" bind:value={fmPassword}
                     placeholder="Generate at fastmail.com → Settings → Security"
                     class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm outline-none
                            focus:border-blue-500 focus:ring-2 focus:ring-blue-100
                            dark:border-gray-600 dark:bg-gray-700 dark:text-gray-100" />
              <p class="mt-1 text-xs text-gray-400 dark:text-gray-600">
                Create at fastmail.com → Settings → Privacy & Security → App Passwords
              </p>
            </div>
            {#if fmError}<p class="text-sm text-red-600 dark:text-red-400">{fmError}</p>{/if}
            <div class="flex gap-2">
              <button onclick={connectFastmail} disabled={fmSaving || !fmEmail || !fmPassword}
                      class="rounded-lg bg-blue-500 px-4 py-2 text-sm font-medium text-white
                             hover:bg-blue-600 disabled:opacity-40">
                {fmSaving ? 'Connecting…' : 'Connect'}
              </button>
              <button onclick={() => { fmShowForm = false; fmError = ''; }}
                      class="rounded-lg border border-gray-200 px-4 py-2 text-sm text-gray-600
                             hover:bg-gray-50 dark:border-gray-600 dark:text-gray-300 dark:hover:bg-gray-700">
                Cancel
              </button>
            </div>
          </div>
        {/if}
      </div>
    {/if}
  </section>
</div>
