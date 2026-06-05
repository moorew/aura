<script lang="ts">
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import { api } from '$lib/api';

  let connected  = $state(false);
  let email      = $state('');
  let labels     = $state<string[]>([]);
  let lastSynced = $state<string | null>(null);

  let syncing   = $state(false);
  let syncResult = $state<{ total: number; new: number; updated: number; errors: number } | null>(null);
  let syncError  = $state('');

  // Inline label editor state
  let labelInput = $state('');
  let savingLabels = $state(false);

  onMount(async () => {
    const cfg = await api.integrations.gmail.get();
    connected  = cfg.connected;
    email      = cfg.email ?? '';
    labels     = cfg.labels ?? ['STARRED'];
    lastSynced = cfg.last_synced_at ?? null;

    // Show success banner if returning from OAuth flow
    const connected_param = $page.url.searchParams.get('connected');
    if (connected_param === '1' && !connected) {
      // reload after redirect
      const fresh = await api.integrations.gmail.get();
      connected  = fresh.connected;
      email      = fresh.email ?? '';
      labels     = fresh.labels ?? ['STARRED'];
    }
  });

  async function sync() {
    syncing = true;
    syncResult = null;
    syncError = '';
    try {
      syncResult = await api.integrations.gmail.sync();
    } catch (e) {
      syncError = (e as Error).message;
    } finally {
      syncing = false;
    }
  }

  async function saveLabels() {
    savingLabels = true;
    try {
      await api.integrations.gmail.updateLabels(labels);
    } finally {
      savingLabels = false;
    }
  }

  function addLabel() {
    const v = labelInput.trim().toUpperCase();
    if (v && !labels.includes(v)) {
      labels = [...labels, v];
      labelInput = '';
    }
  }

  function removeLabel(l: string) {
    labels = labels.filter((x) => x !== l);
  }

  const COMMON_LABELS = ['STARRED', 'INBOX', 'IMPORTANT'];

  async function disconnect() {
    if (!confirm('Disconnect Gmail? Imported tasks will be kept.')) return;
    await api.integrations.gmail.delete();
    connected = false;
    email = '';
  }
</script>

<div class="mx-auto max-w-xl px-6 py-8">
  <!-- Back -->
  <a href="/settings/accounts" class="mb-6 inline-flex items-center gap-1.5 text-sm text-gray-500 hover:text-gray-700">
    <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
      <path stroke-linecap="round" d="m15 18-6-6 6-6"/>
    </svg>
    Integrations
  </a>

  <div class="mb-6 flex items-center gap-3">
    <div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-red-50">
      <svg class="h-5 w-5 text-red-500" viewBox="0 0 24 24" fill="currentColor">
        <path d="M20 4H4c-1.1 0-2 .9-2 2v12c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V6c0-1.1-.9-2-2-2zm0 4-8 5-8-5V6l8 5 8-5v2z"/>
      </svg>
    </div>
    <div>
      <h1 class="text-xl font-semibold text-gray-900">Gmail</h1>
      {#if connected}
        <p class="text-sm text-green-600">{email}</p>
      {:else}
        <p class="text-sm text-gray-400">Not connected</p>
      {/if}
    </div>
  </div>

  {#if !connected}
    <!-- Connect flow -->
    <div class="rounded-xl border border-gray-200 bg-white p-6 text-center">
      <p class="mb-4 text-sm text-gray-600">
        Connect your Gmail account to automatically import starred emails as tasks.
        Sempa only requests read-only access.
      </p>
      <a href={api.integrations.gmail.authUrl()}
         class="inline-flex items-center gap-2 rounded-lg bg-white border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50">
        <svg class="h-4 w-4" viewBox="0 0 24 24" fill="currentColor">
          <path d="M20 4H4c-1.1 0-2 .9-2 2v12c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V6c0-1.1-.9-2-2-2zm0 4-8 5-8-5V6l8 5 8-5v2z"/>
        </svg>
        Connect with Google
      </a>
    </div>
  {:else}
    <!-- Label configuration -->
    <div class="rounded-xl border border-gray-200 bg-white p-5">
      <p class="mb-3 text-sm font-medium text-gray-700">Labels to sync</p>
      <p class="mb-3 text-xs text-gray-400">
        Only emails with these Gmail labels will be imported as tasks.
      </p>

      <!-- Quick-add common labels -->
      <div class="mb-3 flex flex-wrap gap-2">
        {#each COMMON_LABELS as l}
          <button type="button"
                  onclick={() => { if (!labels.includes(l)) labels = [...labels, l]; }}
                  class="rounded-full border px-2.5 py-0.5 text-xs transition-colors
                         {labels.includes(l)
                           ? 'border-blue-300 bg-blue-50 text-blue-700'
                           : 'border-gray-200 text-gray-500 hover:border-gray-300'}">
            {l}
          </button>
        {/each}
      </div>

      <!-- Current labels (custom ones too) -->
      <div class="mb-3 flex flex-wrap gap-2">
        {#each labels as l}
          {#if !COMMON_LABELS.includes(l)}
            <span class="inline-flex items-center gap-1 rounded-full bg-gray-100 px-2.5 py-0.5 text-xs text-gray-700">
              {l}
              <button type="button" onclick={() => removeLabel(l)} class="hover:text-red-500">×</button>
            </span>
          {/if}
        {/each}
      </div>

      <!-- Custom label input -->
      <div class="flex gap-2">
        <input type="text" bind:value={labelInput}
               onkeydown={(e) => e.key === 'Enter' && (e.preventDefault(), addLabel())}
               placeholder="Custom label ID (e.g. Label_12345)"
               class="flex-1 rounded-lg border border-gray-300 px-3 py-1.5 text-sm outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-100" />
        <button type="button" onclick={addLabel}
                class="rounded-lg border border-gray-300 px-3 py-1.5 text-sm text-gray-700 hover:bg-gray-50">
          Add
        </button>
      </div>

      <div class="mt-4 flex items-center gap-3">
        <button type="button" onclick={saveLabels} disabled={savingLabels}
                class="rounded-lg bg-blue-500 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-600 disabled:opacity-50">
          {savingLabels ? 'Saving…' : 'Save labels'}
        </button>

        <button type="button" onclick={sync} disabled={syncing}
                class="rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-50 disabled:opacity-50">
          {syncing ? 'Syncing…' : 'Sync now'}
        </button>
      </div>
    </div>

    <!-- Sync result -->
    {#if syncResult}
      <div class="mt-4 rounded-lg bg-blue-50 px-4 py-3 text-sm text-blue-700">
        Sync complete — {syncResult.new} new, {syncResult.updated} updated, {syncResult.errors} errors
        (total: {syncResult.total})
      </div>
    {/if}

    {#if syncError}
      <div class="mt-4 rounded-lg bg-red-50 px-4 py-3 text-sm text-red-700">{syncError}</div>
    {/if}

    {#if lastSynced}
      <p class="mt-4 text-xs text-gray-400">Last synced: {new Date(lastSynced).toLocaleString()}</p>
    {/if}

    <!-- Danger zone -->
    <div class="mt-10 rounded-xl border border-red-100 bg-red-50 px-5 py-4">
      <p class="mb-3 text-sm font-medium text-red-800">Danger zone</p>
      <button onclick={disconnect}
              class="rounded-lg border border-red-300 px-3 py-1.5 text-sm text-red-700 transition-colors hover:bg-red-100">
        Disconnect Gmail
      </button>
    </div>
  {/if}
</div>
