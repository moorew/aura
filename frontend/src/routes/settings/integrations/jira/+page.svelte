<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api';

  let host      = $state('');
  let email     = $state('');
  let apiToken  = $state('');
  let jql       = $state('');
  let connected = $state(false);
  let lastSynced = $state<string | null>(null);

  let saving   = $state(false);
  let testing  = $state(false);
  let syncing  = $state(false);

  let testStatus = $state<'idle' | 'ok' | 'error'>('idle');
  let testMsg    = $state('');
  let syncResult = $state<{ total: number; new: number; updated: number; errors: number } | null>(null);
  let saveError  = $state('');

  onMount(async () => {
    const cfg = await api.integrations.jira.get();
    connected  = cfg.connected;
    lastSynced = cfg.last_synced_at ?? null;
    if (cfg.config) {
      host     = cfg.config.host;
      email    = cfg.config.email;
      apiToken = cfg.config.api_token ?? '';
      jql      = cfg.config.jql ?? '';
    }
  });

  async function save() {
    saving = true;
    saveError = '';
    try {
      await api.integrations.jira.save({ host, email, api_token: apiToken, jql: jql || undefined });
      connected = true;
    } catch (e) {
      saveError = (e as Error).message;
    } finally {
      saving = false;
    }
  }

  async function test() {
    testing = true;
    testStatus = 'idle';
    testMsg = '';
    try {
      await api.integrations.jira.test();
      testStatus = 'ok';
      testMsg = 'Connection successful';
    } catch (e) {
      testStatus = 'error';
      testMsg = (e as Error).message;
    } finally {
      testing = false;
    }
  }

  async function sync() {
    syncing = true;
    syncResult = null;
    try {
      syncResult = await api.integrations.jira.sync();
    } catch (e) {
      testStatus = 'error';
      testMsg = (e as Error).message;
    } finally {
      syncing = false;
    }
  }

  async function disconnect() {
    if (!confirm('Disconnect Jira? Imported tasks will be kept.')) return;
    await api.integrations.jira.delete();
    connected = false;
    host = email = apiToken = jql = '';
  }
</script>

<div class="mx-auto max-w-xl px-6 py-8">
  <!-- Back -->
  <a href="/settings/accounts" class="mb-6 inline-flex items-center gap-1.5 text-sm transition-colors"
     style="color: var(--sempa-text-soft);">
    <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
      <path stroke-linecap="round" d="m15 18-6-6 6-6"/>
    </svg>
    Settings
  </a>

  <div class="mb-6 flex items-center gap-3">
    <div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg" style="background: var(--sempa-accent-bg);">
      <svg class="h-5 w-5" style="color: var(--sempa-accent);" viewBox="0 0 24 24" fill="currentColor">
        <path d="M11.571 11.513H0a5.218 5.218 0 0 0 5.232 5.215h2.13v2.057A5.215 5.215 0 0 0 12.575 24V12.518a1.005 1.005 0 0 0-1.005-1.005zm5.723-5.756H5.757a5.215 5.215 0 0 0 5.214 5.214h2.129v2.058A5.218 5.218 0 0 0 18.313 18.3V6.763a1.006 1.006 0 0 0-1.019-1.006zM23.277.007H11.749a5.215 5.215 0 0 0 5.214 5.214h2.129v2.058A5.218 5.218 0 0 0 24.282 12.5V1.012A1.005 1.005 0 0 0 23.277.007z"/>
      </svg>
    </div>
    <div>
      <h1 class="text-xl font-semibold" style="color: var(--sempa-text);">Jira</h1>
      {#if connected}
        <p class="text-sm text-green-600 dark:text-green-400">Connected</p>
      {:else}
        <p class="text-sm" style="color: var(--sempa-text-dim);">Not connected</p>
      {/if}
    </div>
  </div>

  <form onsubmit={(e) => { e.preventDefault(); save(); }} class="flex flex-col gap-4">
    <div>
      <label class="mb-1.5 block text-sm font-medium" style="color: var(--sempa-text-soft);" for="host">Jira Host</label>
      <input id="host" type="url" bind:value={host} placeholder="https://yourcompany.atlassian.net"
             class="w-full rounded-lg border px-3 py-2 text-sm outline-none"
             style="border-color: var(--sempa-border); background: var(--sempa-bg-main); color: var(--sempa-text);" />
    </div>

    <div>
      <label class="mb-1.5 block text-sm font-medium" style="color: var(--sempa-text-soft);" for="email">Email</label>
      <input id="email" type="email" bind:value={email} placeholder="you@company.com"
             class="w-full rounded-lg border px-3 py-2 text-sm outline-none"
             style="border-color: var(--sempa-border); background: var(--sempa-bg-main); color: var(--sempa-text);" />
    </div>

    <div>
      <label class="mb-1.5 block text-sm font-medium" style="color: var(--sempa-text-soft);" for="token">API Token</label>
      <input id="token" type="password" bind:value={apiToken} placeholder="Paste your Atlassian API token"
             class="w-full rounded-lg border px-3 py-2 text-sm outline-none"
             style="border-color: var(--sempa-border); background: var(--sempa-bg-main); color: var(--sempa-text);" />
      <p class="mt-1 text-xs" style="color: var(--sempa-text-dim);">
        Generate at <span class="font-mono">id.atlassian.com/manage-profile/security/api-tokens</span>
      </p>
    </div>

    <div>
      <label class="mb-1.5 block text-sm font-medium" style="color: var(--sempa-text-soft);" for="jql">
        JQL Filter <span class="font-normal" style="color: var(--sempa-text-dim);">(optional)</span>
      </label>
      <input id="jql" type="text" bind:value={jql}
             placeholder="assignee = currentUser() AND statusCategory != Done ORDER BY updated DESC"
             class="w-full rounded-lg border px-3 py-2 text-sm outline-none"
             style="border-color: var(--sempa-border); background: var(--sempa-bg-main); color: var(--sempa-text);" />
    </div>

    {#if saveError}
      <p class="text-sm text-red-600 dark:text-red-400">{saveError}</p>
    {/if}

    <div class="flex items-center gap-3 pt-1">
      <button type="submit" disabled={saving}
              class="rounded-lg px-4 py-2 text-sm font-medium text-white transition-colors disabled:opacity-50"
              style="background: var(--sempa-accent);">
        {saving ? 'Saving…' : 'Save'}
      </button>

      {#if connected}
        <button type="button" onclick={test} disabled={testing}
                class="rounded-lg border px-4 py-2 text-sm font-medium transition-colors disabled:opacity-50"
                style="border-color: var(--sempa-border); color: var(--sempa-text-soft);">
          {testing ? 'Testing…' : 'Test connection'}
        </button>

        <button type="button" onclick={sync} disabled={syncing}
                class="rounded-lg border px-4 py-2 text-sm font-medium transition-colors disabled:opacity-50"
                style="border-color: var(--sempa-border); color: var(--sempa-text-soft);">
          {syncing ? 'Syncing…' : 'Sync now'}
        </button>
      {/if}
    </div>
  </form>

  {#if testStatus !== 'idle'}
    <div class="mt-4 rounded-lg px-4 py-3 text-sm
                {testStatus === 'ok' ? 'bg-green-50 text-green-700 dark:bg-green-950 dark:text-green-300' : 'bg-red-50 text-red-700 dark:bg-red-950 dark:text-red-300'}">
      {testMsg}
    </div>
  {/if}

  {#if syncResult}
    <div class="mt-4 rounded-lg px-4 py-3 text-sm" style="background: var(--sempa-accent-bg); color: var(--sempa-accent);">
      Sync complete — {syncResult.new} new, {syncResult.updated} updated, {syncResult.errors} errors
      (total: {syncResult.total})
    </div>
  {/if}

  {#if lastSynced}
    <p class="mt-4 text-xs" style="color: var(--sempa-text-dim);">Last synced: {new Date(lastSynced).toLocaleString()}</p>
  {/if}

  {#if connected}
    <div class="mt-10 rounded-xl border border-red-100 bg-red-50 px-5 py-4 dark:border-red-900 dark:bg-red-950">
      <p class="mb-3 text-sm font-medium text-red-800 dark:text-red-300">Danger zone</p>
      <button onclick={disconnect}
              class="rounded-lg border border-red-300 px-3 py-1.5 text-sm text-red-700 transition-colors hover:bg-red-100 dark:border-red-800 dark:text-red-400 dark:hover:bg-red-900">
        Disconnect Jira
      </button>
    </div>
  {/if}
</div>
