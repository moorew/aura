<script lang="ts">
  import type { Task } from '$lib/types';
  import { formatMinutes } from '$lib/utils';
  import { tagStore } from '$lib/stores/tags.svelte';

  let {
    task, accent,
    onDragStart, onFocusClick, onComplete, onTrash, onClick,
  }: {
    task: Task;
    accent: string;
    onDragStart: (id: string) => void;
    onFocusClick?: (id: string, title: string) => void;
    onComplete?: (id: string) => void;
    onTrash?: (id: string, title: string) => void;
    onClick?: (task: Task) => void;
  } = $props();

  const sourceLabel: Record<string, string> = {
    gmail: 'Gmail', fastmail: 'Mail', jira: 'Jira', google_calendar: 'Cal',
  };
  const isDone      = $derived(task.status === 'done');
  const isRecurring = $derived(!!task.recurrence_origin_id);
  const hasFooter   = $derived(
    !!(task.tags?.length || task.time_estimate_minutes ||
       (task.source && task.source !== 'manual') || isRecurring)
  );
</script>

<div
  draggable="true"
  role="listitem"
  ondragstart={(e) => {
    e.dataTransfer?.setData('application/x-sempa-task', task.id);
    onDragStart(task.id);
  }}
  class="group relative flex flex-col gap-2 rounded-xl border border-gray-100 bg-white p-3
         shadow-sm cursor-grab active:cursor-grabbing active:opacity-60 active:shadow-lg
         transition-all duration-100 hover:border-gray-200 hover:shadow-md
         dark:border-gray-700/40 dark:bg-gray-800 dark:hover:border-gray-600/60"
>
  <div class="flex items-start gap-2.5">
    <!-- Quick-complete circle -->
    <button
      type="button"
      onclick={(e) => { e.stopPropagation(); onComplete?.(task.id); }}
      title={isDone ? 'Completed' : 'Mark complete'}
      class="mt-0.5 h-4 w-4 shrink-0 rounded-full border-2 flex items-center justify-center transition-all cursor-pointer
             {isDone ? 'border-green-500 bg-green-500' : 'border-gray-200 hover:border-green-400 hover:bg-green-50 dark:border-gray-600 dark:hover:border-green-500 dark:hover:bg-green-950'}"
      aria-label="Complete task"
    >
      {#if isDone}
        <svg class="h-2.5 w-2.5 text-white" fill="none" stroke="currentColor" stroke-width="3" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/>
        </svg>
      {/if}
    </button>

    <!-- Title + click to edit -->
    <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions a11y_no_static_element_interactions -->
    <div class="min-w-0 flex-1 cursor-pointer" onclick={() => onClick?.(task)}>
      <p class="text-sm leading-snug
                {isDone
                  ? 'line-through text-gray-300 dark:text-gray-600'
                  : 'font-medium text-gray-800 dark:text-gray-100'}">
        {task.title}
      </p>
    </div>

    <!-- Hover actions -->
    <div class="flex shrink-0 items-center gap-0.5 opacity-0 group-hover:opacity-100 transition-opacity">
      {#if onFocusClick && !isDone}
        <button onclick={(e) => { e.stopPropagation(); onFocusClick?.(task.id, task.title); }}
                class="rounded p-1 text-gray-300 hover:text-amber-500 transition-colors
                       dark:text-gray-600 dark:hover:text-amber-400"
                title="Start focus timer">
          <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
            <circle cx="12" cy="12" r="9"/><path stroke-linecap="round" d="M12 7v5l3 3"/>
          </svg>
        </button>
      {/if}
      {#if onTrash}
        <button onclick={(e) => { e.stopPropagation(); onTrash?.(task.id, task.title); }}
                class="rounded p-1 text-gray-300 hover:text-red-500 transition-colors
                       dark:text-gray-600 dark:hover:text-red-400"
                title="Delete task">
          <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/>
          </svg>
        </button>
      {/if}
    </div>
  </div>

  <!-- Tags + metadata -->
  {#if hasFooter}
    <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
    <div class="flex flex-wrap gap-1 pl-6.5 cursor-pointer" onclick={() => onClick?.(task)}>
      {#each (task.tags ?? []) as tag}
        <span class="rounded-full px-2 py-0.5 text-[10px] font-medium text-white"
              style="background-color: {tagStore.colorFor(tag)}">{tag}</span>
      {/each}
      {#if task.time_estimate_minutes}
        <span class="rounded bg-gray-100 px-1.5 py-0.5 text-[10px] font-mono text-gray-500
                     dark:bg-gray-700/60 dark:text-gray-400">
          {formatMinutes(task.time_estimate_minutes)}
        </span>
      {/if}
      {#if task.source && task.source !== 'manual'}
        <span class="rounded bg-indigo-50 px-1.5 py-0.5 text-[10px] text-indigo-500
                     dark:bg-indigo-950/60 dark:text-indigo-400">
          {sourceLabel[task.source] ?? task.source}
        </span>
      {/if}
      {#if isRecurring}
        <span class="rounded bg-violet-50 px-1.5 py-0.5 text-[10px] text-violet-500
                     dark:bg-violet-950/60 dark:text-violet-400" title="Recurring">↺</span>
      {/if}
    </div>
  {/if}
</div>
