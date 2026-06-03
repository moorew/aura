<script lang="ts">
  import type { Task } from '$lib/types';
  import TaskCard from './TaskCard.svelte';
  import { Plus } from 'lucide-svelte';

  let {
    date,          // YYYY-MM-DD
    dayName,       // "Mon"
    dayNum,        // "3" or "Jun 3"
    isToday,
    tasks,         // all tasks for this day (any status except cancelled), sorted by position
    isDragOver,
    onTaskDragStart,
    onTaskFocusClick,
    onTaskComplete,
    onTaskClick,
    onDrop,
    onEmailDrop,
    onDragOver,
    onDragLeave,
    onAddClick,
  }: {
    date: string; dayName: string; dayNum: string; isToday: boolean;
    tasks: Task[]; isDragOver: boolean;
    onTaskDragStart: (id: string) => void;
    onTaskFocusClick?: (id: string, title: string) => void;
    onTaskComplete?: (id: string) => void;
    onTaskClick?: (task: Task) => void;
    onDrop: (date: string, insertIndex?: number) => void;
    onEmailDrop?: (emailData: { id: string; subject: string }, date: string) => void;
    onDragOver: (date: string) => void;
    onDragLeave: () => void;
    onAddClick: (date: string) => void;
  } = $props();

  const active = $derived(tasks.filter(t => t.status !== 'done').sort((a, b) => a.position - b.position));
  const done   = $derived(tasks.filter(t => t.status === 'done').sort((a, b) => a.position - b.position));
  let showDone = $state(false);

  let taskListEl = $state<HTMLElement | undefined>();
  let insertIdx  = $state<number | null>(null);

  function calcInsertIdx(e: DragEvent): number {
    if (!taskListEl) return active.length;
    const els = Array.from(taskListEl.querySelectorAll('[data-task-idx]')) as HTMLElement[];
    for (let i = 0; i < els.length; i++) {
      const rect = els[i].getBoundingClientRect();
      if (e.clientY < rect.top + rect.height / 2) return i;
    }
    return active.length;
  }
</script>

<div class="flex w-52 shrink-0 flex-col"
     ondragover={(e) => { e.preventDefault(); insertIdx = calcInsertIdx(e); onDragOver(date); }}
     ondragleave={(e) => {
       if (!(e.currentTarget as HTMLElement).contains(e.relatedTarget as Node)) {
         insertIdx = null; onDragLeave();
       }
     }}
     ondrop={(e) => {
       e.preventDefault();
       const emailData = e.dataTransfer?.getData('application/x-sempa-email');
       if (emailData) {
         try { onEmailDrop?.(JSON.parse(emailData), date); } catch {}
       } else {
         onDrop(date, insertIdx ?? undefined);
       }
       insertIdx = null;
     }}>

  <!-- Column header -->
  <div class="mb-2.5 rounded-xl px-3 py-2.5 text-center transition-colors
              {isToday ? 'text-white shadow-sm' : 'bg-gray-100/70 dark:bg-gray-800/40'}"
       style={isToday ? 'background:var(--a500)' : ''}>
    <p class="text-[10px] font-semibold uppercase tracking-widest
              {isToday ? 'text-white/70' : 'text-gray-400 dark:text-gray-500'}">
      {dayName}
    </p>
    <p class="text-lg font-bold leading-tight
              {isToday ? 'text-white' : 'text-gray-700 dark:text-gray-200'}">
      {dayNum}
    </p>
    {#if tasks.length > 0}
      <p class="text-[10px] {isToday ? 'text-white/60' : 'text-gray-400 dark:text-gray-600'}">
        {done.length}/{tasks.length}
      </p>
    {/if}
  </div>

  <!-- Task area -->
  <div class="relative flex flex-1 flex-col rounded-2xl transition-all duration-150
              {isDragOver
                ? 'ring-2 ring-offset-1 dark:ring-offset-gray-950'
                : 'bg-gray-100/50 dark:bg-gray-800/20'}"
       style={isDragOver ? 'background:var(--a50);ring-color:var(--a400)' : ''}>

    <!-- Priority gradient — very subtle "start here" visual at top -->
    <div class="pointer-events-none absolute inset-x-0 top-0 h-16 rounded-t-2xl"
         style="background:linear-gradient(to bottom, var(--a50) 0%, transparent 100%); opacity:0.6">
    </div>

    <div role="list" bind:this={taskListEl}
         class="relative flex flex-col gap-2 overflow-y-auto p-2
                [scrollbar-width:thin] [scrollbar-color:theme(colors.gray.200)_transparent]">

      {#each active as task, i (task.id)}
        {#if isDragOver && insertIdx === i}
          <div class="h-0.5 rounded-full mx-1" style="background:var(--a400)"></div>
        {/if}
        <div data-task-idx={i}>
          <TaskCard {task} accent="bg-gray-400"
                   onDragStart={onTaskDragStart}
                   onFocusClick={onTaskFocusClick}
                   onComplete={onTaskComplete}
                   onClick={onTaskClick} />
        </div>
      {/each}

      {#if isDragOver && insertIdx === active.length}
        <div class="h-0.5 rounded-full mx-1" style="background:var(--a400)"></div>
      {/if}
      {#if active.length === 0 && !isDragOver}
        <div class="min-h-[60px]"></div>
      {/if}
    </div>

    <!-- Done tasks (collapsible) -->
    {#if done.length > 0}
      <div class="px-2 pb-1">
        <button onclick={() => showDone = !showDone}
                class="flex w-full items-center gap-1 rounded-lg px-2 py-1 text-[10px]
                       text-gray-400 hover:bg-white/60 transition-colors dark:text-gray-600 dark:hover:bg-gray-700/30">
          <svg class="h-3 w-3 transition-transform {showDone ? 'rotate-180' : ''}"
               fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
            <path stroke-linecap="round" d="M19 9l-7 7-7-7"/>
          </svg>
          {done.length} done
        </button>
        {#if showDone}
          <div class="flex flex-col gap-1.5 pt-1">
            {#each done as task (task.id)}
              <TaskCard {task} accent="bg-green-400"
                       onDragStart={onTaskDragStart}
                       onComplete={onTaskComplete}
                       onClick={onTaskClick} />
            {/each}
          </div>
        {/if}
      </div>
    {/if}

    <!-- Add task -->
    <button onclick={() => onAddClick(date)}
            class="flex items-center gap-1.5 rounded-b-2xl px-3 py-2.5 text-xs text-gray-400
                   hover:bg-white/60 hover:text-gray-600 transition-colors
                   dark:text-gray-600 dark:hover:bg-gray-700/30 dark:hover:text-gray-400">
      <Plus size={12} />
      Add task
    </button>
  </div>
</div>
