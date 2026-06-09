<script lang="ts">
  import { api } from '$lib/api';
  import type { Objective, Task } from '$lib/types';
  import { weekStart } from '$lib/utils';

  let { date }: { date: string } = $props();

  const ws = $derived(weekStart(date));
  let objectives = $state<Objective[]>([]);
  let tasks      = $state<Task[]>([]);
  let collapsed  = $state(false);

  $effect(() => { ws; void load(); });

  async function load() {
    try {
      [objectives, tasks] = await Promise.all([
        api.objectives.listByWeek(ws),
        api.tasks.listByWeek(ws),
      ]);
    } catch { /* ignore */ }
  }

  function objTasks(id: string) {
    return tasks.filter(t => t.weekly_objective_id === id && t.status !== 'cancelled');
  }
  function objDone(id: string)  { return objTasks(id).filter(t => t.status === 'done').length; }
  function objTotal(id: string) { return objTasks(id).length; }
  function objPct(id: string)   {
    const t = objTotal(id); return t === 0 ? 0 : Math.round(objDone(id) / t * 100);
  }

  const totalDone = $derived(objectives.filter(o => o.status === 'completed').length);
  const total     = $derived(objectives.length);
  const overallPct = $derived(total === 0 ? 0 : Math.round(totalDone / total * 100));
</script>

{#if total > 0}
  <div class="border-b border-gray-100 dark:border-gray-800/60">
    <!-- Header row -->
    <button onclick={() => collapsed = !collapsed}
            class="flex w-full items-center justify-between px-4 py-2.5 text-left
                   hover:bg-gray-50/50 dark:hover:bg-gray-800/30 transition-colors">
      <div class="flex items-center gap-2">
        <span class="text-[10.5px] font-semibold uppercase tracking-wider text-gray-400 dark:text-gray-600">
          This week
        </span>
        <span class="text-[10.5px] text-gray-400 dark:text-gray-600">
          {totalDone}/{total}
        </span>
      </div>
      <div class="flex items-center gap-2">
        <div class="h-1 w-14 overflow-hidden rounded-full bg-gray-100 dark:bg-gray-800">
          <div class="h-full rounded-full bg-[var(--a500)] transition-all"
               style="width:{overallPct}%"></div>
        </div>
        <svg class="h-3 w-3 text-gray-300 transition-transform dark:text-gray-600 {collapsed ? '-rotate-90' : ''}"
             fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
          <path stroke-linecap="round" d="M19 9l-7 7-7-7"/>
        </svg>
      </div>
    </button>

    {#if !collapsed}
      <div class="px-4 pb-2.5 space-y-1">
        {#each objectives as obj (obj.id)}
          {@const p = objPct(obj.id)}
          {@const done = obj.status === 'completed'}
          <a href="/week/{ws}"
             class="flex items-center gap-2 rounded-lg py-1 px-1 hover:bg-gray-50 dark:hover:bg-gray-800/40 transition-colors">
            <div class="h-1.5 w-1.5 shrink-0 rounded-full
                        {done ? 'bg-green-400' : p === 100 ? 'bg-green-400' : 'bg-[var(--a400)]'}"></div>
            <span class="flex-1 truncate text-xs {done ? 'line-through text-gray-400 dark:text-gray-600' : 'text-gray-600 dark:text-gray-400'}">
              {obj.title}
            </span>
            <span class="shrink-0 text-[10.5px] font-mono {done || p === 100 ? 'text-green-500' : 'text-gray-400 dark:text-gray-600'}">
              {p}%
            </span>
          </a>
        {/each}
      </div>
    {/if}
  </div>
{/if}
