<script lang="ts">
  import { onMount } from 'svelte';
  import { RefreshCw } from 'lucide-svelte';
  import { api } from '$lib/api';
  import type { FastmailEmail } from '$lib/types';

  let emails    = $state<FastmailEmail[]>([]);
  let loading   = $state(true);
  let error     = $state('');
  let connected = $state(true);

  // Per-email action state
  let converting = $state<Record<string, boolean>>({});
  let archiving  = $state<Record<string, boolean>>({});
  let done       = $state<Record<string, boolean>>({});

  onMount(load);

  async function load() {
    loading = true; error = '';
    try {
      emails = await api.integrations.fastmail.emails();
    } catch (e: any) {
      if (e.message?.includes('not connected')) {
        connected = false;
      } else {
        error = e.message ?? 'Failed to load emails';
      }
    } finally {
      loading = false;
    }
  }

  async function toTask(email: FastmailEmail) {
    converting[email.id] = true;
    try {
      await api.integrations.fastmail.toTask(email.id, email.subject);
      done[email.id] = true;
      // Fade out after a moment
      setTimeout(() => { emails = emails.filter(e => e.id !== email.id); }, 600);
    } catch (e: any) {
      error = e.message;
    } finally {
      converting[email.id] = false;
    }
  }

  async function archive(email: FastmailEmail) {
    archiving[email.id] = true;
    try {
      await api.integrations.fastmail.archive(email.id);
      emails = emails.filter(e => e.id !== email.id);
    } catch (e: any) {
      error = e.message;
    } finally {
      archiving[email.id] = false;
    }
  }

  function senderName(email: FastmailEmail): string {
    if (!email.from?.length) return '?';
    return email.from[0].name || email.from[0].email;
  }

  function senderInitial(email: FastmailEmail): string {
    return senderName(email).charAt(0).toUpperCase();
  }

  function formatTime(iso: string): string {
    const d = new Date(iso);
    const now = new Date();
    const diffDays = Math.floor((now.getTime() - d.getTime()) / 86400000);
    if (diffDays === 0) return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
    if (diffDays === 1) return 'Yesterday';
    if (diffDays < 7)  return d.toLocaleDateString([], { weekday: 'short' });
    return d.toLocaleDateString([], { month: 'short', day: 'numeric' });
  }
</script>

<svelte:head><title>Inbox — Sempa</title></svelte:head>

<div class="flex h-full flex-col">
  <!-- Header -->
  <header class="flex items-center justify-between border-b border-gray-200 bg-white px-6 py-4
                 dark:border-gray-800 dark:bg-gray-900">
    <div>
      <h1 class="text-base font-semibold text-gray-900 dark:text-gray-50">Fastmail inbox</h1>
      {#if !loading}
        <p class="text-xs text-gray-400 dark:text-gray-600">{emails.length} message{emails.length !== 1 ? 's' : ''}</p>
      {/if}
    </div>
    <button onclick={load} disabled={loading}
            class="flex items-center gap-1.5 text-xs font-medium transition-opacity hover:opacity-80 disabled:opacity-40"
            style="border: 1px solid var(--sempa-border); border-radius: 8px; padding: 7px 14px;
                   color: var(--sempa-text-soft); background: transparent;">
      <RefreshCw size={13} class={loading ? 'animate-spin' : ''} />
      {loading ? 'Loading…' : 'Refresh'}
    </button>
  </header>

  <!-- Body -->
  <div class="flex-1 overflow-y-auto">
    {#if !connected}
      <div class="flex h-full flex-col items-center justify-center gap-3 text-center px-6">
        <div class="flex h-12 w-12 items-center justify-center rounded-full bg-blue-50 dark:bg-blue-950">
          <svg class="h-6 w-6 text-blue-500" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24">
            <path stroke-linecap="round" d="M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"/>
          </svg>
        </div>
        <p class="text-sm font-medium text-gray-700 dark:text-gray-300">Fastmail not connected</p>
        <p class="text-xs text-gray-400 dark:text-gray-600">Connect your Fastmail account in Settings → Accounts.</p>
        <a href="/settings/accounts"
           class="mt-1 rounded-lg bg-blue-500 px-4 py-2 text-sm font-medium text-white hover:bg-blue-600">
          Go to Settings
        </a>
      </div>

    {:else if loading}
      <div class="divide-y divide-gray-100 dark:divide-gray-800">
        {#each Array(8) as _}
          <div class="flex items-start gap-3 px-6 py-4 animate-pulse">
            <div class="h-9 w-9 shrink-0 rounded-full bg-gray-100 dark:bg-gray-800"></div>
            <div class="flex-1 space-y-2 pt-0.5">
              <div class="h-3 w-32 rounded bg-gray-100 dark:bg-gray-800"></div>
              <div class="h-3 w-64 rounded bg-gray-100 dark:bg-gray-800"></div>
              <div class="h-3 w-48 rounded bg-gray-50 dark:bg-gray-800/50"></div>
            </div>
          </div>
        {/each}
      </div>

    {:else if error}
      <div class="m-6 rounded-xl border border-red-200 bg-red-50 p-4 text-sm text-red-700
                  dark:border-red-900 dark:bg-red-950 dark:text-red-400">
        {error}
        <button onclick={load} class="ml-2 underline">Retry</button>
      </div>

    {:else if emails.length === 0}
      <div class="flex h-full flex-col items-center justify-center gap-2 text-center">
        <svg class="h-10 w-10 text-gray-200 dark:text-gray-800" fill="none" stroke="currentColor" stroke-width="1.5" viewBox="0 0 24 24">
          <path stroke-linecap="round" d="M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"/>
        </svg>
        <p class="text-sm text-gray-400 dark:text-gray-600">Inbox is empty</p>
      </div>

    {:else}
      <ul class="divide-y divide-gray-100 dark:divide-gray-800">
        {#each emails as email (email.id)}
          <li class="group flex items-start gap-3 px-6 py-3.5 transition-colors
                     {done[email.id] ? 'opacity-40' : 'hover:bg-gray-50 dark:hover:bg-gray-800/50'}">

            <!-- Avatar -->
            <div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-full text-xs font-semibold text-white
                        {email.is_unread ? 'bg-blue-500' : 'bg-gray-300 dark:bg-gray-600'}">
              {senderInitial(email)}
            </div>

            <!-- Content -->
            <div class="min-w-0 flex-1">
              <div class="flex items-baseline justify-between gap-2">
                <span class="truncate text-sm {email.is_unread ? 'font-semibold text-gray-900 dark:text-gray-50' : 'text-gray-700 dark:text-gray-300'}">
                  {senderName(email)}
                </span>
                <span class="shrink-0 text-xs text-gray-400 dark:text-gray-600">{formatTime(email.received_at)}</span>
              </div>
              <p class="truncate text-sm {email.is_unread ? 'font-medium text-gray-800 dark:text-gray-100' : 'text-gray-600 dark:text-gray-400'}">
                {email.subject || '(no subject)'}
              </p>
              {#if email.preview}
                <p class="truncate text-xs text-gray-400 dark:text-gray-600 mt-0.5">{email.preview}</p>
              {/if}
            </div>

            <!-- Actions — visible on hover -->
            <div class="flex shrink-0 items-center gap-1.5 opacity-0 group-hover:opacity-100 transition-opacity">
              <button onclick={() => toTask(email)}
                      disabled={converting[email.id] || done[email.id]}
                      title="Add to today's tasks and archive"
                      class="flex items-center gap-1 rounded-lg bg-blue-500 px-2.5 py-1.5 text-xs font-medium text-white
                             hover:bg-blue-600 disabled:opacity-50 transition-colors">
                {#if converting[email.id]}
                  <svg class="h-3 w-3 animate-spin" fill="none" viewBox="0 0 24 24">
                    <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
                    <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8z"/>
                  </svg>
                {:else if done[email.id]}
                  ✓
                {:else}
                  → Task
                {/if}
              </button>
              <button onclick={() => archive(email)}
                      disabled={archiving[email.id]}
                      title="Archive"
                      class="rounded-lg border border-gray-200 px-2.5 py-1.5 text-xs text-gray-500
                             hover:bg-gray-100 disabled:opacity-50 transition-colors
                             dark:border-gray-700 dark:text-gray-400 dark:hover:bg-gray-700">
                {archiving[email.id] ? '…' : 'Archive'}
              </button>
            </div>
          </li>
        {/each}
      </ul>
    {/if}
  </div>
</div>
