<script lang="ts">
  import type { Task, TaskStatus } from '$lib/types';
  import TaskCard from './TaskCard.svelte';

  let {
    label, status, tasks, accent, isDragOver,
    onTaskDragStart, onTaskFocusClick, onTaskComplete, onTaskClick,
    onDrop, onEmailDrop, onDragOver, onDragLeave, onAddClick,
  }: {
    label: string; status: TaskStatus; tasks: Task[];
    accent: string; isDragOver: boolean;
    onTaskDragStart: (id: string) => void;
    onTaskFocusClick?: (id: string, title: string) => void;
    onTaskComplete?: (id: string) => void;
    onTaskClick?: (task: Task) => void;
    onDrop: (status: TaskStatus, insertIndex?: number) => void;
    onEmailDrop?: (emailData: { id: string; subject: string }, status: TaskStatus) => void;
    onDragOver: (status: TaskStatus) => void;
    onDragLeave: () => void;
    onAddClick: (status: TaskStatus) => void;
  } = $props();

  let taskListEl = $state<HTMLElement | undefined>();
  let insertIdx  = $state<number | null>(null);

  function calcInsertIdx(e: DragEvent): number {
    if (!taskListEl) return tasks.length;
    const els = Array.from(taskListEl.querySelectorAll('[data-task-idx]')) as HTMLElement[];
    for (let i = 0; i < els.length; i++) {
      const rect = els[i].getBoundingClientRect();
      if (e.clientY < rect.top + rect.height / 2) return i;
    }
    return tasks.length;
  }
</script>

<div role="region" aria-label="{label} column"
     class="flex w-64 shrink-0 flex-col"
     ondragover={(e) => { e.preventDefault(); insertIdx = calcInsertIdx(e); onDragOver(status); }}
     ondragleave={(e) => {
       if (!(e.currentTarget as HTMLElement).contains(e.relatedTarget as Node)) {
         insertIdx = null; onDragLeave();
       }
     }}
     ondrop={(e) => {
       e.preventDefault();
       const emailData = e.dataTransfer?.getData('application/x-sempa-email');
       if (emailData) {
         try { onEmailDrop?.(JSON.parse(emailData), status); } catch {}
       } else {
         onDrop(status, insertIdx ?? undefined);
       }
       insertIdx = null;
     }}>

  <!-- Column header -->
  <div class="mb-3 flex items-center justify-between px-1">
    <div class="flex items-center gap-2">
      <div class="h-2 w-2 rounded-full {accent}"></div>
      <span class="text-xs font-semibold uppercase tracking-widest text-gray-500 dark:text-gray-400">
        {label}
      </span>
    </div>
    <span class="rounded-full bg-gray-100 px-2 py-0.5 text-xs font-mono text-gray-400
                 dark:bg-gray-800 dark:text-gray-600">
      {tasks.length}
    </span>
  </div>

  <!-- Drop zone wrapper -->
  <div class="flex flex-1 flex-col rounded-2xl transition-all duration-150
              {isDragOver
                ? 'bg-blue-50/70 ring-2 ring-blue-400/40 ring-offset-1 dark:bg-blue-950/30 dark:ring-blue-700/50'
                : 'bg-gray-100/60 dark:bg-gray-800/30'}">

    <div role="list" bind:this={taskListEl}
         class="flex flex-col gap-2 overflow-y-auto p-2
                [scrollbar-width:thin] [scrollbar-color:theme(colors.gray.200)_transparent]
                dark:[scrollbar-color:theme(colors.gray.700)_transparent]">

      {#each tasks as task, i (task.id)}
        {#if isDragOver && insertIdx === i}
          <div class="h-0.5 rounded-full bg-blue-400 dark:bg-blue-500 mx-1"></div>
        {/if}
        <div data-task-idx={i}>
          <TaskCard {task} {accent}
                   onDragStart={onTaskDragStart}
                   onFocusClick={onTaskFocusClick}
                   onComplete={onTaskComplete}
                   onClick={onTaskClick} />
        </div>
      {/each}

      {#if isDragOver && insertIdx === tasks.length}
        <div class="h-0.5 rounded-full bg-blue-400 dark:bg-blue-500 mx-1"></div>
      {/if}
      {#if isDragOver && tasks.length === 0}
        <div class="flex h-14 items-center justify-center rounded-xl border-2 border-dashed
                    border-blue-300 text-xs text-blue-400 dark:border-blue-700 dark:text-blue-600">
          Drop here
        </div>
      {/if}
      {#if tasks.length === 0 && !isDragOver}
        <div class="min-h-[80px]"></div>
      {/if}
    </div>

    {#if status !== 'done'}
      <button onclick={() => onAddClick(status)}
              class="flex items-center gap-1.5 rounded-b-2xl px-3 py-2.5 text-xs text-gray-400 transition-colors
                     hover:bg-white/60 hover:text-gray-600 dark:text-gray-600 dark:hover:bg-gray-700/30 dark:hover:text-gray-400">
        <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" d="M12 4v16m8-8H4"/>
        </svg>
        Add task
      </button>
    {/if}
  </div>
</div>
