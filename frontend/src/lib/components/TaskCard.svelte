<script lang="ts">
  import type { Task } from '$lib/types';
  import { formatMinutes, today as getToday, bareUrl, prettyUrl } from '$lib/utils';
  import { tagStore } from '$lib/stores/tags.svelte';
  import { hapticClick } from '$lib/haptics';
  import { api } from '$lib/api';
  import { mobile } from '$lib/stores/mobile.svelte';

  let {
    task, accent,
    onDragStart, onFocusClick, onComplete, onTrash, onClick, onFocusMode, onHover,
  }: {
    task: Task;
    accent: string;
    onDragStart: (id: string) => void;
    onFocusClick?: (id: string, title: string) => void;
    onComplete?: (id: string) => void;
    onTrash?: (id: string, title: string) => void;
    onClick?: (task: Task) => void;
    onFocusMode?: (id: string) => void;
    onHover?: (id: string | null) => void;
  } = $props();

  const todayStr = getToday();

  const sourceLabel: Record<string, string> = {
    gmail: 'Gmail', fastmail: 'Mail', jira: 'Jira', google_calendar: 'Cal',
  };
  const isDone      = $derived(task.status === 'done');
  const isRecurring = $derived(!!task.recurrence_origin_id);

  // Reminder marker — a clean bell + time on the card so you can see at a glance
  // which tasks will ring, without opening them. Shown for any task with a
  // reminder that hasn't been completed. Past-due reminders (already fired) read
  // dimmer so upcoming ones stand out.
  const hasReminder = $derived(!!task.remind_at && !isDone);
  const remindLabel = $derived.by(() => {
    if (!task.remind_at) return '';
    const dt = new Date(task.remind_at);
    if (isNaN(dt.getTime())) return '';
    const ymd = (d: Date) =>
      `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
    const time = dt.toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' });
    // Same day → just the time; another day → short date so it's unambiguous.
    return ymd(dt) === todayStr ? time : `${dt.toLocaleDateString([], { month: 'short', day: 'numeric' })} ${time}`;
  });
  const remindPast = $derived(!!task.remind_at && new Date(task.remind_at).getTime() < Date.now());

  const daysBehind = $derived.by(() => {
    if (!task.planned_date || task.status === 'done' || task.status === 'cancelled') return 0;
    if (task.planned_date >= todayStr) return 0;
    const past = new Date(task.planned_date + 'T12:00:00').getTime();
    const now  = new Date(todayStr       + 'T12:00:00').getTime();
    return Math.round((now - past) / 86400000);
  });

  const hasFooter = $derived(
    !!(task.tags?.length || task.time_estimate_minutes ||
       (task.source && task.source !== 'manual') || isRecurring || daysBehind > 0 || hasReminder)
  );

  // Streak: count consecutive done instances of this recurring task (most-recent first)
  let streak = $state(0);

  $effect(() => {
    const originId = task.recurrence_origin_id;
    if (!originId) return;
    api.tasks.listByRecurrenceOrigin(originId).then(siblings => {
      const sorted = [...siblings].sort((a, b) =>
        (b.planned_date ?? '').localeCompare(a.planned_date ?? '')
      );
      let count = 0;
      for (const t of sorted) {
        if (t.status === 'done') count++;
        else if (count === 0) continue; // skip current pending instance at the top
        else break;
      }
      streak = count;
    }).catch(() => {});
  });
</script>

<div
  draggable="true"
  role="listitem"
  ondragstart={(e) => {
    e.dataTransfer?.setData('application/x-sempa-task', task.id);
    onDragStart(task.id);
  }}
  onmouseenter={() => onHover?.(task.id)}
  onmouseleave={() => onHover?.(null)}
  class="group relative flex flex-col gap-2 rounded-[10px] shadow-sm cursor-grab
         active:cursor-grabbing active:scale-[0.98] active:shadow-none
         transition-all duration-100 hover:shadow-md min-h-[44px]"
  style="padding: 9px 10px; background: var(--card-bg); border: 1px solid var(--card-border);"
>
  <div class="flex items-start gap-2">
    <!-- Quick-complete — 44×44 tap target on mobile, compact on desktop so the
         title gets the full column width instead of being squashed (FIX 3) -->
    <button
      type="button"
      onclick={(e) => { e.stopPropagation(); hapticClick(); onComplete?.(task.id); }}
      title={isDone ? 'Completed' : 'Mark complete'}
      class="shrink-0 flex items-center justify-center cursor-pointer
             {mobile.value ? 'h-[44px] w-[44px] -ml-1 -my-1' : 'h-[18px] w-[18px] mt-px'}"
      aria-label="Complete task"
    >
      <span class="flex h-4 w-4 items-center justify-center rounded-full border-2 transition-all
                   {isDone ? 'border-green-500 bg-green-500' : 'border-gray-200 hover:border-green-400 hover:bg-green-50 dark:border-gray-600 dark:hover:border-green-500 dark:hover:bg-green-950'}">
        {#if isDone}
          <svg class="h-2.5 w-2.5 text-white" fill="none" stroke="currentColor" stroke-width="3" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/>
          </svg>
        {/if}
      </span>
    </button>

    <!-- Title + click to edit -->
    <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions a11y_no_static_element_interactions -->
    <div class="min-w-0 flex-1 cursor-pointer" onclick={() => onClick?.(task)}>
      <p class="{isDone ? 'line-through' : ''}"
         style="font-size: 13px; font-weight: 500; line-height: 1.35; letter-spacing: -0.005em;
                text-wrap: pretty; overflow-wrap: anywhere; word-break: break-word;
                color: {isDone ? 'var(--sempa-text-dim)' : 'var(--sempa-text)'};">
        {#if bareUrl(task.title)}
          <span class="inline-flex max-w-full items-center gap-1 align-middle">
            <svg class="h-3 w-3 shrink-0 opacity-60" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" d="M10 13a5 5 0 007.07 0l3-3a5 5 0 00-7.07-7.07l-1.72 1.71M14 11a5 5 0 00-7.07 0l-3 3a5 5 0 007.07 7.07l1.71-1.71"/>
            </svg>
            <span class="truncate">{prettyUrl(bareUrl(task.title)!)}</span>
          </span>
        {:else}
          {task.title}
        {/if}
      </p>
    </div>

    <!-- Hover/mobile actions (FIX 2) — on desktop these overlay the card's
         top-right corner (absolute) so they don't steal width from the title;
         on mobile they stay inline with full-size touch targets. -->
    <div class="flex items-center gap-0.5 transition-opacity
                {mobile.value
                  ? 'shrink-0 opacity-100'
                  : 'absolute right-1.5 top-1.5 z-10 rounded-md shadow-sm opacity-0 group-hover:opacity-100'}"
         style={mobile.value ? '' : 'background: var(--card-bg); padding: 1px;'}>
      {#if onFocusMode && !isDone}
        <button onclick={(e) => { e.stopPropagation(); onFocusMode?.(task.id); }}
                class="{mobile.value ? 'h-[44px] w-[44px] flex items-center justify-center' : 'rounded p-1'}
                       text-gray-300 hover:text-[var(--sempa-accent)] transition-colors
                       dark:text-gray-600 dark:hover:text-[var(--sempa-accent)]"
                title="Focus mode">
          <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" d="M15 3h6v6M9 21H3v-6M21 3l-7 7M3 21l7-7"/>
          </svg>
        </button>
      {/if}
      {#if onFocusClick && !isDone}
        <button onclick={(e) => { e.stopPropagation(); onFocusClick?.(task.id, task.title); }}
                class="{mobile.value ? 'h-[44px] w-[44px] flex items-center justify-center' : 'rounded p-1'}
                       text-gray-300 hover:text-amber-500 transition-colors
                       dark:text-gray-600 dark:hover:text-amber-400"
                title="Start focus timer">
          <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
            <circle cx="12" cy="12" r="9"/><path stroke-linecap="round" d="M12 7v5l3 3"/>
          </svg>
        </button>
      {/if}
      {#if onTrash}
        <button onclick={(e) => { e.stopPropagation(); onTrash?.(task.id, task.title); }}
                class="{mobile.value ? 'h-[44px] w-[44px] flex items-center justify-center' : 'rounded p-1'}
                       text-gray-300 hover:text-red-500 transition-colors
                       dark:text-gray-600 dark:hover:text-red-400"
                title="Delete task">
          <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/>
          </svg>
        </button>
      {/if}
      {#if mobile.value}
        <!-- 6-dot drag handle (FIX 6c) -->
        <div
          onpointerdown={(e) => {
            (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
            onDragStart(task.id);
          }}
          class="flex h-[44px] w-[44px] cursor-grab items-center justify-center text-gray-300 dark:text-gray-600"
          aria-label="Drag to reorder"
          role="button"
          tabindex="-1"
        >
          <svg class="h-4 w-4" viewBox="0 0 16 16" fill="currentColor">
            <circle cx="5" cy="4" r="1.5"/><circle cx="11" cy="4" r="1.5"/>
            <circle cx="5" cy="8" r="1.5"/><circle cx="11" cy="8" r="1.5"/>
            <circle cx="5" cy="12" r="1.5"/><circle cx="11" cy="12" r="1.5"/>
          </svg>
        </div>
      {/if}
    </div>
  </div>

  <!-- Tags + metadata — full card width UNDER the title (not indented under the
       checkbox), so badges flow horizontally instead of stacking. -->
  {#if hasFooter}
    <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
    <div class="flex flex-wrap items-center cursor-pointer" style="gap: 4px; margin-top: 7px;" onclick={() => onClick?.(task)}>
      {#if (task.tags ?? []).length}
        <!-- Tags as colour dots only — scan the list by colour without label
             clutter. Full names live in the task detail view. Title shows the
             names on hover/long-press for accessibility. -->
        <span class="inline-flex items-center" style="gap: 3px;" title={(task.tags ?? []).join(', ')}>
          {#each task.tags ?? [] as tag}
            <span class="shrink-0 rounded-full" style="width: 7px; height: 7px; background-color: {tagStore.colorFor(tag)};"></span>
          {/each}
        </span>
      {/if}
      {#if hasReminder}
        <span class="type-badge inline-flex items-center rounded"
              style="gap: 3px; padding: 2px 7px;
                     {remindPast
                       ? 'background: color-mix(in srgb, var(--sempa-text) 6%, transparent); color: var(--sempa-text-dim);'
                       : 'background: var(--sempa-accent-bg); color: var(--sempa-accent);'}"
              title={remindPast ? `Reminder was due ${remindLabel}` : `Reminder ${remindLabel}`}>
          <svg width="9" height="9" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.25" style="flex: 0 0 auto;">
            <path stroke-linecap="round" stroke-linejoin="round" d="M18 8a6 6 0 0 0-12 0c0 7-3 9-3 9h18s-3-2-3-9M13.73 21a2 2 0 0 1-3.46 0"/>
          </svg>
          {remindLabel}
        </span>
      {/if}
      {#if task.source && task.source !== 'manual'}
        <span class="type-badge rounded" style="padding: 2px 7px; background: var(--sempa-accent-bg); color: var(--sempa-accent);">
          {sourceLabel[task.source] ?? task.source}
        </span>
      {/if}
      {#if task.time_estimate_minutes}
        <span class="type-badge rounded" style="padding: 2px 7px; background: color-mix(in srgb, var(--sempa-text) 6%, transparent); color: var(--sempa-text-soft);">
          {formatMinutes(task.time_estimate_minutes)}
        </span>
      {/if}
      {#if isRecurring}
        <span class="type-badge rounded" style="padding: 2px 7px; background: var(--sempa-accent-bg); color: var(--sempa-accent);"
              title="Recurring{streak > 0 ? ` · ${streak} in a row` : ''}">
          ↺{#if streak > 0}&thinsp;{streak}🔥{/if}
        </span>
      {/if}
      {#if daysBehind > 0}
        <span class="type-badge rounded bg-amber-50 text-amber-600 dark:bg-amber-950/60 dark:text-amber-400"
              style="padding: 2px 7px;"
              title="{daysBehind} day{daysBehind !== 1 ? 's' : ''} overdue">
          +{daysBehind}d
        </span>
      {/if}
    </div>
  {/if}
</div>
