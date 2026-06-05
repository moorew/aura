<script lang="ts">
  import { api } from '$lib/api';
  import type { Task } from '$lib/types';
  import { appendPosition } from '$lib/utils';
  import { mobile } from '$lib/stores/mobile.svelte';

  let { parentId, parentDate }: { parentId: string; parentDate?: string } = $props();

  let subtasks  = $state<Task[]>([]);
  let loading   = $state(true);
  let newTitle  = $state('');
  let adding    = $state(false);
  let inputEl   = $state<HTMLInputElement | undefined>();

  $effect(() => {
    parentId; void load();
  });

  async function load() {
    loading = true;
    try { subtasks = await api.tasks.listByParent(parentId); }
    catch { /* ignore */ }
    finally { loading = false; }
  }

  async function add() {
    const t = newTitle.trim();
    if (!t) return;
    adding = true;
    try {
      const newPos = appendPosition(subtasks.map(s => s.position));
      const sub = await api.tasks.create({
        title: t,
        parent_task_id: parentId,
        status: 'planned',
        planned_date: parentDate,
        position: newPos,
      });
      subtasks = [...subtasks, sub];
      newTitle = '';
      inputEl?.focus();
    } catch { /* ignore */ }
    finally { adding = false; }
  }

  async function toggleDone(sub: Task) {
    const newStatus = sub.status === 'done' ? 'planned' : 'done';
    subtasks = subtasks.map(s => s.id === sub.id ? { ...s, status: newStatus } : s);
    try {
      const updated = await api.tasks.update(sub.id, {
        status: newStatus,
        completed_at: newStatus === 'done' ? new Date().toISOString() : null,
      });
      subtasks = subtasks.map(s => s.id === updated.id ? updated : s);
    } catch { await load(); }
  }

  async function remove(id: string) {
    subtasks = subtasks.filter(s => s.id !== id);
    await api.tasks.delete(id);
  }

  const done  = $derived(subtasks.filter(s => s.status === 'done').length);
  const total = $derived(subtasks.length);
</script>

<div class="space-y-2">
  <div class="flex items-center justify-between">
    <span class="text-xs font-medium text-gray-600 dark:text-gray-400">
      Sub-tasks {#if total > 0}<span class="text-gray-400 dark:text-gray-600">({done}/{total})</span>{/if}
    </span>
    {#if total > 0}
      <div class="h-1 w-20 rounded-full bg-gray-200 dark:bg-gray-700 overflow-hidden">
        <div class="h-full rounded-full bg-green-400 transition-all"
             style="width: {total ? (done / total) * 100 : 0}%"></div>
      </div>
    {/if}
  </div>

  {#if loading}
    <div class="space-y-1.5">
      {#each Array(2) as _}
        <div class="h-7 rounded-lg bg-gray-100 dark:bg-gray-800 animate-pulse"></div>
      {/each}
    </div>
  {:else}
    <ul class="space-y-1">
      {#each subtasks as sub (sub.id)}
        <li class="group flex items-center gap-2 rounded-lg px-2 py-1.5 hover:bg-gray-50 dark:hover:bg-gray-800/60">
          <button onclick={() => toggleDone(sub)}
                  class="h-4 w-4 shrink-0 rounded-full border-2 flex items-center justify-center transition-all
                         {sub.status === 'done'
                           ? 'border-green-500 bg-green-500'
                           : 'border-gray-300 hover:border-green-400 dark:border-gray-600'}">
            {#if sub.status === 'done'}
              <svg class="h-2.5 w-2.5 text-white" fill="none" stroke="currentColor" stroke-width="3" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/>
              </svg>
            {/if}
          </button>
          <span class="flex-1 text-sm {sub.status === 'done' ? 'line-through text-gray-400 dark:text-gray-600' : 'text-gray-700 dark:text-gray-200'}">
            {sub.title}
          </span>
          <button onclick={() => remove(sub.id)}
                  aria-label="Delete sub-task"
                  class="text-gray-300 hover:text-red-400 transition-all dark:text-gray-600 dark:hover:text-red-400
                         {mobile.value ? 'opacity-100' : 'opacity-0 group-hover:opacity-100'}">
            <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
              <path stroke-linecap="round" d="M6 18L18 6M6 6l12 12"/>
            </svg>
          </button>
        </li>
      {/each}
    </ul>
  {/if}

  <!-- Add sub-task input -->
  <div class="flex items-center gap-2 rounded-lg border border-dashed border-gray-200
              px-2 py-1.5 focus-within:border-blue-400 focus-within:bg-blue-50/30
              dark:border-gray-700 dark:focus-within:border-blue-600 dark:focus-within:bg-blue-950/20">
    <svg class="h-3.5 w-3.5 shrink-0 text-gray-300 dark:text-gray-600" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
      <path stroke-linecap="round" d="M12 4v16m8-8H4"/>
    </svg>
    <input bind:this={inputEl}
           bind:value={newTitle}
           onkeydown={(e) => { if (e.key === 'Enter') { e.preventDefault(); void add(); } }}
           type="text"
           placeholder="Add a sub-task…"
           class="flex-1 bg-transparent text-xs text-gray-700 placeholder-gray-400 outline-none
                  dark:text-gray-200 dark:placeholder-gray-600" />
    {#if newTitle.trim()}
      <button onclick={add} disabled={adding}
              class="text-xs text-blue-500 hover:text-blue-700 disabled:opacity-40 dark:text-blue-400 dark:hover:text-blue-300">
        Add
      </button>
    {/if}
  </div>
</div>
