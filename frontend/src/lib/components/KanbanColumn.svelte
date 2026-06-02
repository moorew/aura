<script lang="ts">
  import type { Task, TaskStatus } from '$lib/types';
  import TaskCard from './TaskCard.svelte';

  let {
    label, status, tasks, accent, bg, border, isDragOver,
    onTaskDragStart, onTaskFocusClick, onTaskComplete, onTaskClick,
    onDrop, onEmailDrop, onDragOver, onDragLeave, onAddClick,
  }: {
    label: string; status: TaskStatus; tasks: Task[];
    accent: string; bg: string; border: string; isDragOver: boolean;
    onTaskDragStart: (id: string) => void;
    onTaskFocusClick?: (id: string, title: string) => void;
    onTaskComplete?: (id: string) => void;
    onTaskClick?: (task: Task) => void;
    onDrop: (status: TaskStatus) => void;
    onEmailDrop?: (emailData: { id: string; subject: string }, status: TaskStatus) => void;
    onDragOver: (status: TaskStatus) => void;
    onDragLeave: () => void;
    onAddClick: (status: TaskStatus) => void;
  } = $props();
</script>

<div role="region" aria-label="{label} column"
     class="flex w-64 shrink-0 flex-col rounded-xl border {border} {bg} transition-colors
            dark:border-opacity-50 {isDragOver ? 'ring-2 ring-blue-400 ring-offset-1' : ''}"
     ondragover={(e) => { e.preventDefault(); onDragOver(status); }}
     ondragleave={onDragLeave}
     ondrop={(e) => {
       e.preventDefault();
       const emailData = e.dataTransfer?.getData('application/x-sempa-email');
       if (emailData) {
         try { onEmailDrop?.(JSON.parse(emailData), status); } catch {}
       } else {
         onDrop(status);
       }
     }}>

  <div class="flex items-center justify-between px-3 py-2.5 border-b {border} dark:border-opacity-50">
    <div class="flex items-center gap-2">
      <div class="h-2 w-2 rounded-full {accent}"></div>
      <span class="text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400">{label}</span>
    </div>
    <span class="rounded-full bg-white border {border} px-2 py-0.5 text-xs text-gray-400 font-mono
                 dark:bg-gray-800 dark:border-gray-700 dark:text-gray-500">
      {tasks.length}
    </span>
  </div>

  <div role="list" class="flex flex-1 flex-col gap-2 overflow-y-auto p-2 min-h-[120px]">
    {#each tasks as task (task.id)}
      <TaskCard {task} {accent}
               onDragStart={onTaskDragStart}
               onFocusClick={onTaskFocusClick}
               onComplete={onTaskComplete}
               onClick={onTaskClick} />
    {/each}
    {#if isDragOver && tasks.length === 0}
      <div class="flex h-16 items-center justify-center rounded-lg border-2 border-dashed border-blue-300 text-xs text-blue-400 dark:border-blue-700 dark:text-blue-600">
        Drop here
      </div>
    {/if}
  </div>

  {#if status !== 'done'}
    <button onclick={() => onAddClick(status)}
            class="flex items-center gap-1.5 px-3 py-2.5 text-xs text-gray-400 hover:text-gray-600
                   hover:bg-white/60 rounded-b-xl transition-colors border-t {border}
                   dark:border-opacity-50 dark:text-gray-500 dark:hover:text-gray-300 dark:hover:bg-gray-700/40">
      <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" d="M12 4v16m8-8H4"/>
      </svg>
      Add task
    </button>
  {/if}
</div>
