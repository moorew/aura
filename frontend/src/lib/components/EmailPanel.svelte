<script lang="ts">
  import { api } from '$lib/api';
  import type { FastmailEmail } from '$lib/types';

  let {
    onTaskCreated,
  }: {
    onTaskCreated?: (task: import('$lib/types').Task) => void;
  } = $props();

  let emails    = $state<FastmailEmail[]>([]);
  let loading   = $state(true);
  let error     = $state('');
  let connected = $state(true);

  let converting = $state<Record<string, boolean>>({});
  let archiving  = $state<Record<string, boolean>>({});
  let done       = $state<Record<string, string>>({});  // id → 'task' | 'archive'

  $effect(() => { load(); });

  async function load() {
    loading = true; error = '';
    try {
      emails = await api.integrations.fastmail.emails();
    } catch (e: any) {
      if (e.message?.includes('not connected')) connected = false;
      else error = e.message ?? 'Failed';
    } finally { loading = false; }
  }

  export async function toTask(email: FastmailEmail) {
    if (converting[email.id]) return;
    converting[email.id] = true;
    try {
      const task = await api.integrations.fastmail.toTask(email.id, email.subject);
      done[email.id] = 'task';
      onTaskCreated?.(task);
      setTimeout(() => { emails = emails.filter(e => e.id !== email.id); }, 500);
    } catch (e: any) { error = e.message; }
    finally { converting[email.id] = false; }
  }

  async function archive(email: FastmailEmail) {
    archiving[email.id] = true;
    try {
      await api.integrations.fastmail.archive(email.id);
      emails = emails.filter(e => e.id !== email.id);
    } catch { /* ignore */ }
    finally { archiving[email.id] = false; }
  }

  // Called by the parent when an email is dropped on a kanban column
  export function removeEmail(id: string) {
    emails = emails.filter(e => e.id !== id);
  }

  function senderName(e: FastmailEmail) {
    return e.from?.[0]?.name || e.from?.[0]?.email || '?';
  }

  function senderInitial(e: FastmailEmail) {
    return senderName(e).charAt(0).toUpperCase();
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

  function onDragStart(e: DragEvent, email: FastmailEmail) {
    e.dataTransfer?.setData('application/x-sempa-email',
      JSON.stringify({ id: email.id, subject: email.subject }));
    e.dataTransfer!.effectAllowed = 'copy';
  }
</script>

<div class="flex h-full flex-col bg-white dark:bg-gray-900 border-l border-gray-200 dark:border-gray-800">
  <!-- Header -->
  <div class="flex items-center justify-between px-4 py-3 border-b border-gray-200 dark:border-gray-800">
    <span class="text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400">
      Fastmail inbox
    </span>
    <button onclick={load} disabled={loading}
            class="text-xs text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 disabled:opacity-40">
      {loading ? '…' : 'Refresh'}
    </button>
  </div>

  <div class="flex-1 overflow-y-auto">
    {#if !connected}
      <div class="flex flex-col items-center justify-center gap-2 p-6 text-center">
        <p class="text-xs text-gray-400 dark:text-gray-600">Fastmail not connected</p>
        <a href="/settings/accounts" class="text-xs text-blue-500 hover:underline">Set up →</a>
      </div>

    {:else if loading}
      <div class="space-y-px">
        {#each Array(6) as _}
          <div class="flex items-start gap-2.5 px-3 py-3 animate-pulse">
            <div class="h-7 w-7 shrink-0 rounded-full bg-gray-100 dark:bg-gray-800"></div>
            <div class="flex-1 space-y-1.5 pt-0.5">
              <div class="h-2.5 w-24 rounded bg-gray-100 dark:bg-gray-800"></div>
              <div class="h-2.5 w-full rounded bg-gray-100 dark:bg-gray-800"></div>
            </div>
          </div>
        {/each}
      </div>

    {:else if error}
      <p class="p-4 text-xs text-red-500 dark:text-red-400">{error}</p>

    {:else if emails.length === 0}
      <div class="flex h-32 items-center justify-center">
        <p class="text-xs text-gray-400 dark:text-gray-600">Inbox is empty</p>
      </div>

    {:else}
      <ul class="divide-y divide-gray-100 dark:divide-gray-800">
        {#each emails as email (email.id)}
          <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
          <li class="group relative cursor-grab px-3 py-3 transition-colors
                     {done[email.id] ? 'opacity-40' : 'hover:bg-gray-50 dark:hover:bg-gray-800/60'}
                     active:cursor-grabbing"
              draggable="true"
              ondragstart={(e) => onDragStart(e, email)}>

            <div class="flex items-start gap-2.5">
              <!-- Avatar -->
              <div class="flex h-7 w-7 shrink-0 items-center justify-center rounded-full text-xs font-semibold text-white
                          {email.is_unread ? 'bg-blue-500' : 'bg-gray-300 dark:bg-gray-600'}">
                {senderInitial(email)}
              </div>

              <!-- Content -->
              <div class="min-w-0 flex-1 pr-1">
                <div class="flex items-baseline justify-between gap-1">
                  <span class="truncate text-xs {email.is_unread ? 'font-semibold text-gray-800 dark:text-gray-100' : 'text-gray-600 dark:text-gray-400'}">
                    {senderName(email)}
                  </span>
                  <span class="shrink-0 text-xs text-gray-400 dark:text-gray-600">{formatTime(email.received_at)}</span>
                </div>
                <p class="truncate text-xs {email.is_unread ? 'font-medium text-gray-700 dark:text-gray-200' : 'text-gray-500 dark:text-gray-500'}">
                  {email.subject || '(no subject)'}
                </p>
              </div>
            </div>

            <!-- Hover actions -->
            <div class="absolute right-2 top-1/2 -translate-y-1/2 flex items-center gap-1
                        opacity-0 group-hover:opacity-100 transition-opacity">
              <button onclick={() => toTask(email)}
                      disabled={!!converting[email.id] || !!done[email.id]}
                      title="Add as task + archive"
                      class="rounded bg-blue-500 px-2 py-1 text-xs font-medium text-white
                             hover:bg-blue-600 disabled:opacity-50 transition-colors">
                {converting[email.id] ? '…' : done[email.id] === 'task' ? '✓' : '→ Task'}
              </button>
              <button onclick={() => archive(email)}
                      disabled={archiving[email.id]}
                      title="Archive"
                      class="rounded border border-gray-200 px-1.5 py-1 text-xs text-gray-400
                             hover:text-gray-600 hover:bg-gray-100 disabled:opacity-50
                             dark:border-gray-700 dark:hover:bg-gray-700 transition-colors">
                {archiving[email.id] ? '…' : '×'}
              </button>
            </div>
          </li>
        {/each}
      </ul>
    {/if}
  </div>

  <div class="border-t border-gray-100 px-3 py-2 dark:border-gray-800">
    <p class="text-xs text-gray-400 dark:text-gray-600">
      Drag emails onto a task column, or click → Task
    </p>
  </div>
</div>
