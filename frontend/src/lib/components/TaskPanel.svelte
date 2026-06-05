<script lang="ts">
  import type { Objective, PomodoroSession, Task, TaskStatus } from '$lib/types';
  import { tagStore } from '$lib/stores/tags.svelte';
  import { api } from '$lib/api';
  import { weekStart as calcWeekStart } from '$lib/utils';
  import SubTaskList from './SubTaskList.svelte';
  import SempaSelect from '$lib/components/ui/SempaSelect.svelte';
  import { mobile } from '$lib/stores/mobile.svelte';
  import { onMount } from 'svelte';

  const TIME_OPTIONS = [
    { label: 'No estimate',  value: null  },
    { label: '15 min',       value: 15    },
    { label: '30 min',       value: 30    },
    { label: '45 min',       value: 45    },
    { label: '1 hour',       value: 60    },
    { label: '1.5 hours',    value: 90    },
    { label: '2 hours',      value: 120   },
    { label: '2.5 hours',    value: 150   },
    { label: '3 hours',      value: 180   },
    { label: '4 hours',      value: 240   },
    { label: '6 hours',      value: 360   },
    { label: '8 hours',      value: 480   },
  ];

  const DAYS = ['Sunday','Monday','Tuesday','Wednesday','Thursday','Friday','Saturday'];
  const ordinal = (n: number) => {
    const s = ['th','st','nd','rd'], v = n % 100;
    return s[(v - 20) % 10] || s[v] || s[0];
  };

  let {
    open,
    task = null,          // null = create mode; Task = edit mode
    defaultStatus = 'planned' as TaskStatus,
    defaultDate,
    onSave,
    onClose,
    inline = false,       // when true, renders content only (no overlay/aside wrapper)
  }: {
    open: boolean;
    task?: Task | null;
    defaultStatus?: TaskStatus;
    defaultDate: string;
    onSave: (task: Task) => void;
    onClose: () => void;
    inline?: boolean;
  } = $props();

  const isEdit = $derived(task !== null);

  // Form state
  let title = $state('');
  let description = $state('');
  let plannedDate = $state('');
  let estimateMinutes = $state<number | null>(null);
  let actualMinutesInput = $state('');
  // Split date+time state (FIX 4 — datetime-local broken on Android)
  let scheduledStartDate = $state('');
  let scheduledStartTime = $state('');
  let scheduledEndDate   = $state('');
  let scheduledEndTime   = $state('');

  // Mobile bottom sheet state (FIX 5)
  let sheetMaxHeight  = $state(600);
  let dragDeltaY      = $state(0);
  let draggingSheet   = $state(false);
  let sheetTouchStartY = $state(0);

  onMount(() => {
    function updateHeight() {
      sheetMaxHeight = Math.round((window.visualViewport?.height ?? window.innerHeight) * 0.92);
    }
    window.visualViewport?.addEventListener('resize', updateHeight);
    updateHeight();
    return () => window.visualViewport?.removeEventListener('resize', updateHeight);
  });

  function sheetTouchStart(e: TouchEvent) {
    sheetTouchStartY = e.touches[0].clientY;
    dragDeltaY = 0;
    draggingSheet = true;
  }
  function sheetTouchMove(e: TouchEvent) {
    if (!draggingSheet) return;
    dragDeltaY = Math.max(0, e.touches[0].clientY - sheetTouchStartY);
  }
  function sheetTouchEnd() {
    if (!draggingSheet) return;
    draggingSheet = false;
    if (dragDeltaY > 80) onClose();
    dragDeltaY = 0;
  }
  let selectedObjectiveId = $state<string | null>(null);
  let weekObjectives = $state<Objective[]>([]);
  let recurrenceRule = $state('');
  let selectedTags = $state<string[]>([]);
  let tagSearch = $state('');
  let tagDropdownOpen = $state(false);
  let saving = $state(false);
  let error = $state('');
  let titleInput: HTMLInputElement | undefined = $state();

  let sessions = $state<PomodoroSession[]>([]);

  $effect(() => {
    if (!open || !task) { sessions = []; return; }
    api.pomodoros.listByTask(task.id).then(s => { sessions = s; }).catch(() => {});
  });

  // FIX 4 helpers — split/combine for separate date+time inputs
  function splitFromISO(iso: string | null | undefined): { date: string; time: string } {
    if (!iso) return { date: '', time: '' };
    const local = iso.substring(0, 16); // treat stored value as local-ish
    const [date, time] = local.split('T');
    return { date: date ?? '', time: time ?? '' };
  }
  function combineToISO(date: string, time: string): string | null {
    if (!date) return null;
    const t = time || '00:00';
    return new Date(`${date}T${t}`).toISOString();
  }

  const recurrenceOptions = $derived.by(() => {
    const d = new Date((plannedDate || defaultDate) + 'T12:00:00');
    const wd = d.getDay(), dom = d.getDate();
    return [
      { value: '',               label: "Doesn't repeat" },
      { value: 'daily',          label: 'Every day' },
      { value: 'weekdays',       label: 'Every weekday (Mon–Fri)' },
      { value: `weekly:${wd}`,   label: `Weekly on ${DAYS[wd]}` },
      { value: `monthly:${dom}`, label: `Monthly on the ${dom}${ordinal(dom)}` },
    ];
  });

  // Populate form when panel opens / task changes
  $effect(() => {
    if (!open) return;
    if (task) {
      title = task.title;
      description = task.description ?? '';
      plannedDate = task.planned_date ?? defaultDate;
      estimateMinutes = task.time_estimate_minutes ?? null;
      actualMinutesInput = task.time_actual_minutes ? String(task.time_actual_minutes) : '';
      const ss = splitFromISO(task.scheduled_start);
      scheduledStartDate = ss.date; scheduledStartTime = ss.time;
      const se = splitFromISO(task.scheduled_end);
      scheduledEndDate = se.date; scheduledEndTime = se.time;
      recurrenceRule = task.recurrence_rule ?? '';
      selectedTags = [...(task.tags ?? [])];
      selectedObjectiveId = task.weekly_objective_id ?? null;
    } else {
      title = ''; description = ''; plannedDate = defaultDate;
      estimateMinutes = null; actualMinutesInput = '';
      scheduledStartDate = ''; scheduledStartTime = '';
      scheduledEndDate = '';   scheduledEndTime = '';
      recurrenceRule = ''; selectedTags = [];
      selectedObjectiveId = null;
    }
    tagSearch = ''; tagDropdownOpen = false; error = '';
    setTimeout(() => titleInput?.focus(), 30);

    // Load objectives for the current week
    const dateForWeek = task?.planned_date ?? defaultDate;
    if (dateForWeek) {
      const ws = calcWeekStart(dateForWeek);
      api.objectives.listByWeek(ws).then(objs => { weekObjectives = objs; }).catch(() => {});
    }
  });

  // Tags
  const filteredTags = $derived(
    tagStore.definitions.filter(t =>
      !selectedTags.includes(t.name) &&
      t.name.toLowerCase().includes(tagSearch.toLowerCase())
    )
  );

  function toggleTag(name: string) {
    name = name.toLowerCase();
    if (selectedTags.includes(name)) selectedTags = selectedTags.filter(t => t !== name);
    else selectedTags = [...selectedTags, name];
    tagSearch = '';
  }

  function handleTagKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') { tagDropdownOpen = false; return; }
    if (e.key === 'Enter' && tagSearch.trim()) {
      e.preventDefault();
      const match = filteredTags[0];
      if (match) toggleTag(match.name);
      else toggleTag(tagSearch.trim());
    }
    if (e.key === 'Backspace' && tagSearch === '' && selectedTags.length) {
      selectedTags = selectedTags.slice(0, -1);
    }
  }

  async function handleSubmit() {
    if (!title.trim()) return;
    saving = true; error = '';
    try {
      let saved: Task;
      if (isEdit && task) {
        const actualMin = actualMinutesInput.trim() ? parseInt(actualMinutesInput, 10) || null : null;
        saved = await api.tasks.update(task.id, {
          title: title.trim(),
          description: description.trim() || null,
          planned_date: recurrenceRule ? null : (plannedDate || null),
          time_estimate_minutes: estimateMinutes ?? null,
          time_actual_minutes: actualMin,
          tags: selectedTags,
          scheduled_start: combineToISO(scheduledStartDate, scheduledStartTime),
          scheduled_end:   combineToISO(scheduledEndDate,   scheduledEndTime),
          weekly_objective_id: selectedObjectiveId ?? null,
        });
      } else {
        saved = await api.tasks.create({
          title: title.trim(),
          description: description.trim() || undefined,
          tags: selectedTags,
          ...(recurrenceRule
            ? { recurrence_rule: recurrenceRule }
            : {
                status: defaultStatus,
                planned_date: plannedDate || undefined,
              }),
          time_estimate_minutes: estimateMinutes ?? undefined,
          weekly_objective_id: selectedObjectiveId ?? undefined,
        });
      }
      onSave(saved);
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to save task';
    } finally {
      saving = false;
    }
  }

  const sourceLabel: Record<string, string> = {
    gmail: 'Gmail', fastmail: 'Fastmail', jira: 'Jira', google_calendar: 'Calendar'
  };
</script>

{#snippet panelContent()}
    <!-- Header — clear the status bar on the desktop right-side drawer (fixed top-0).
         The mobile sheet enters from the bottom, so it needs no top inset. -->
    <div class="flex items-center justify-between border-b border-gray-100 px-5 py-4 dark:border-gray-800"
         style={!mobile.value && !inline ? 'padding-top: max(12px, calc(env(safe-area-inset-top, 0px) + 8px));' : ''}>
      <div>
        <h2 class="text-sm font-semibold text-gray-800 dark:text-gray-100">
          {isEdit ? 'Edit task' : 'New task'}
        </h2>
        {#if isEdit && task?.source && task.source !== 'manual'}
          <p class="text-xs text-gray-400 dark:text-gray-600">
            From {sourceLabel[task.source] ?? task.source}
            {#if task.source_url}
              · <a href={task.source_url} target="_blank" rel="noopener"
                   class="hover:underline text-blue-500">Open original ↗</a>
            {/if}
          </p>
        {/if}
      </div>
      <button onclick={onClose}
              class="rounded-lg p-1.5 text-gray-400 hover:bg-gray-100 hover:text-gray-600 transition-colors
                     dark:text-gray-500 dark:hover:bg-gray-800 dark:hover:text-gray-300">
        <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
          <path stroke-linecap="round" d="M6 18L18 6M6 6l12 12"/>
        </svg>
      </button>
    </div>

    <!-- Body -->
    <div class="flex-1 overflow-y-auto px-5 py-4 space-y-4">

      <!-- Title -->
      <div>
        <label class="mb-1.5 block text-xs font-medium text-gray-600 dark:text-gray-400" for="task-title">
          Title <span class="text-red-400">*</span>
        </label>
        <input id="task-title"
               bind:this={titleInput}
               bind:value={title}
               onkeydown={(e) => e.key === 'Escape' && onClose()}
               type="text"
               placeholder="What needs to get done?"
               class="w-full rounded-lg border border-gray-200 bg-gray-50 px-3 py-2.5 text-sm
                      text-gray-800 placeholder-gray-400 outline-none
                      focus:border-blue-400 focus:bg-white focus:ring-2 focus:ring-blue-100
                      dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100 dark:placeholder-gray-600
                      dark:focus:border-blue-500 dark:focus:bg-gray-800 dark:focus:ring-blue-900/40" />
      </div>

      <!-- Notes -->
      <div>
        <label class="mb-1.5 block text-xs font-medium text-gray-600 dark:text-gray-400" for="task-notes">
          Notes <span class="text-xs font-normal text-gray-400 dark:text-gray-600">— markdown supported</span>
        </label>
        <textarea id="task-notes" bind:value={description} rows="4"
                  placeholder="Add details, links, context...&#10;&#10;Supports **bold**, _italic_, [links](https://...)"
                  class="w-full resize-none rounded-lg border border-gray-200 bg-gray-50 px-3 py-2.5 text-sm
                         text-gray-800 placeholder-gray-400 outline-none leading-relaxed
                         focus:border-blue-400 focus:bg-white focus:ring-2 focus:ring-blue-100
                         dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100 dark:placeholder-gray-600
                         dark:focus:border-blue-500 dark:focus:bg-gray-800"></textarea>
      </div>

      <!-- Links extracted from email -->
      {#if isEdit && task?.source_metadata}
        {@const links = (() => { try { return JSON.parse(task.source_metadata ?? '{}').links ?? []; } catch { return []; } })()}
        {#if links.length > 0}
          <div>
            <p class="mb-1.5 text-xs font-medium text-gray-600 dark:text-gray-400">Links from email</p>
            <div class="flex flex-wrap gap-1.5">
              {#each links as link}
                <a href={link} target="_blank" rel="noopener noreferrer"
                   class="inline-flex items-center gap-1 rounded-full bg-blue-50 px-2.5 py-1 text-xs text-blue-600
                          hover:bg-blue-100 dark:bg-blue-950 dark:text-blue-400 dark:hover:bg-blue-900 truncate max-w-full">
                  <svg class="h-3 w-3 shrink-0" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                    <path stroke-linecap="round" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14"/>
                  </svg>
                  <span class="truncate">{new URL(link).hostname}</span>
                </a>
              {/each}
            </div>
          </div>
        {/if}
      {/if}

      <!-- Date + Estimate -->
      <div class="grid grid-cols-2 gap-3">
        <div>
          <label class="mb-1.5 block text-xs font-medium text-gray-600 dark:text-gray-400" for="task-date">
            Due date
          </label>
          <input id="task-date" type="date" bind:value={plannedDate}
                 disabled={!!recurrenceRule}
                 class="w-full rounded-lg border border-gray-200 bg-gray-50 px-3 py-2.5 text-sm
                        text-gray-800 outline-none focus:border-blue-400 focus:ring-2 focus:ring-blue-100
                        disabled:opacity-40 disabled:cursor-not-allowed
                        dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100
                        [color-scheme:light] dark:[color-scheme:dark]" />
        </div>
        <div>
          <label class="mb-1.5 block text-xs font-medium text-gray-600 dark:text-gray-400" for="task-estimate">
            Time estimate
          </label>
          <SempaSelect id="task-estimate" bind:value={estimateMinutes}
                       placeholder="No estimate"
                       options={TIME_OPTIONS.map(o => ({ value: o.value, label: o.label }))} />
        </div>
      </div>

      <!-- Weekly objective -->
      {#if weekObjectives.length > 0}
        <div>
          <label class="mb-1.5 block text-xs font-medium text-gray-600 dark:text-gray-400" for="task-objective">
            Weekly objective
          </label>
          <SempaSelect id="task-objective" bind:value={selectedObjectiveId}
                       placeholder="No objective"
                       options={[{ value: null, label: 'No objective' },
                                 ...weekObjectives.map(o => ({ value: o.id, label: o.title, icon: '🎯' }))]} />
        </div>
      {/if}

      <!-- Tags -->
      <div>
        <label class="mb-1.5 block text-xs font-medium text-gray-600 dark:text-gray-400">Tags</label>
        <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
        <div class="flex min-h-[42px] flex-wrap gap-1.5 items-center rounded-lg border border-gray-200 bg-gray-50 px-3 py-2
                    focus-within:border-blue-400 focus-within:ring-2 focus-within:ring-blue-100
                    dark:border-gray-700 dark:bg-gray-800 dark:focus-within:border-blue-500"
             onclick={() => tagDropdownOpen = true}>
          {#each selectedTags as tag}
            <span class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium text-white shrink-0"
                  style="background-color: {tagStore.colorFor(tag)}">
              {tag}
              <button type="button" onclick={(e) => { e.stopPropagation(); selectedTags = selectedTags.filter(t => t !== tag); }}
                      class="opacity-75 hover:opacity-100 ml-0.5">×</button>
            </span>
          {/each}
          <input bind:value={tagSearch}
                 onfocus={() => tagDropdownOpen = true}
                 onblur={() => setTimeout(() => tagDropdownOpen = false, 300)}
                 onkeydown={handleTagKeydown}
                 type="text"
                 placeholder={selectedTags.length ? '' : 'Search or add tags…'}
                 class="flex-1 min-w-[80px] bg-transparent text-sm text-gray-700 placeholder-gray-400 outline-none
                        dark:text-gray-200 dark:placeholder-gray-600" />
        </div>
        {#if tagDropdownOpen}
          <div class="relative z-10">
            <div class="absolute top-1 left-0 right-0 rounded-lg border border-gray-200 bg-white shadow-lg
                        dark:border-gray-700 dark:bg-gray-800 max-h-44 overflow-y-auto">
              {#if filteredTags.length}
                {#each filteredTags as t}
                  <button type="button"
                          onmousedown={(e) => { e.preventDefault(); toggleTag(t.name); }}
                          class="flex w-full items-center gap-2.5 px-3 py-2 text-sm text-left
                                 hover:bg-gray-50 dark:hover:bg-gray-700">
                    <span class="h-3 w-3 rounded-full shrink-0" style="background-color: {t.color}"></span>
                    <span class="text-gray-700 dark:text-gray-200">{t.name}</span>
                  </button>
                {/each}
              {:else if tagSearch.trim()}
                <button type="button"
                        onmousedown={(e) => { e.preventDefault(); toggleTag(tagSearch.trim()); tagSearch = ''; }}
                        class="flex w-full items-center gap-2 px-3 py-2 text-sm
                               text-gray-500 hover:bg-gray-50 dark:text-gray-400 dark:hover:bg-gray-700">
                  <span class="text-blue-500">+</span> Create "<strong>{tagSearch.trim()}</strong>"
                </button>
              {:else}
                <p class="px-3 py-2 text-sm text-gray-400 dark:text-gray-600">No tags yet — type to create</p>
              {/if}
            </div>
          </div>
        {/if}
      </div>

      <!-- Scheduled time (edit mode only) — split date+time inputs for Android (FIX 4) -->
      {#if isEdit}
        <div>
          <label class="mb-1.5 block text-xs font-medium text-gray-600 dark:text-gray-400">
            Scheduled time <span class="font-normal text-gray-400 dark:text-gray-600">— drag to calendar or set here</span>
          </label>
          <div class="grid grid-cols-2 gap-2">
            <div class="space-y-1.5">
              <label class="block text-[10px] text-gray-400 dark:text-gray-600" for="sched-start-date">Start date</label>
              <input id="sched-start-date" type="date" bind:value={scheduledStartDate}
                     class="w-full rounded-lg border border-gray-200 bg-gray-50 px-2 py-2 text-xs
                            text-gray-800 outline-none focus:border-blue-400 focus:ring-2 focus:ring-blue-100
                            dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100
                            [color-scheme:light] dark:[color-scheme:dark]" />
              {#if scheduledStartDate}
                <input id="sched-start-time" type="time" bind:value={scheduledStartTime}
                       class="w-full rounded-lg border border-gray-200 bg-gray-50 px-2 py-2 text-xs
                              text-gray-800 outline-none focus:border-blue-400 focus:ring-2 focus:ring-blue-100
                              dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100
                              [color-scheme:light] dark:[color-scheme:dark]" />
              {/if}
            </div>
            <div class="space-y-1.5">
              <label class="block text-[10px] text-gray-400 dark:text-gray-600" for="sched-end-date">End date</label>
              <input id="sched-end-date" type="date" bind:value={scheduledEndDate}
                     class="w-full rounded-lg border border-gray-200 bg-gray-50 px-2 py-2 text-xs
                            text-gray-800 outline-none focus:border-blue-400 focus:ring-2 focus:ring-blue-100
                            dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100
                            [color-scheme:light] dark:[color-scheme:dark]" />
              {#if scheduledEndDate}
                <input id="sched-end-time" type="time" bind:value={scheduledEndTime}
                       class="w-full rounded-lg border border-gray-200 bg-gray-50 px-2 py-2 text-xs
                              text-gray-800 outline-none focus:border-blue-400 focus:ring-2 focus:ring-blue-100
                              dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100
                              [color-scheme:light] dark:[color-scheme:dark]" />
              {/if}
            </div>
          </div>
          {#if scheduledStartDate}
            <button onclick={() => { scheduledStartDate = ''; scheduledStartTime = ''; scheduledEndDate = ''; scheduledEndTime = ''; }}
                    class="mt-1 text-xs text-gray-400 hover:text-red-500 dark:text-gray-600 dark:hover:text-red-400">
              × Clear schedule
            </button>
          {/if}
        </div>
      {/if}

      <!-- Log actual time (edit mode only) -->
      {#if isEdit}
        <div>
          <label class="mb-1.5 block text-xs font-medium text-gray-600 dark:text-gray-400" for="task-actual">
            Actual time logged
          </label>
          <div class="flex items-center gap-2">
            <input id="task-actual" type="number" min="0" bind:value={actualMinutesInput}
                   placeholder="minutes"
                   class="w-28 rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 text-sm
                          text-gray-800 outline-none focus:border-blue-400 focus:ring-2 focus:ring-blue-100
                          dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100" />
            <span class="text-xs text-gray-400 dark:text-gray-600">minutes
              {#if parseInt(actualMinutesInput) > 0}
                ({Math.floor(parseInt(actualMinutesInput) / 60)}h {parseInt(actualMinutesInput) % 60}m)
              {/if}
            </span>
          </div>
          <p class="mt-1 text-[10px] text-gray-400 dark:text-gray-600">
            Updated automatically by pomodoro sessions
          </p>
        </div>
      {/if}

      <!-- Sub-tasks (edit mode only) -->
      {#if isEdit && task}
        <div>
          <SubTaskList parentId={task.id} parentDate={task.planned_date ?? undefined} />
        </div>
      {/if}

      <!-- Pomodoro session history (edit mode only) -->
      {#if isEdit && task && sessions.length > 0}
        <div>
          <p class="mb-1.5 text-xs font-medium text-gray-600 dark:text-gray-400">
            Focus sessions
            <span class="ml-1 font-normal text-gray-400 dark:text-gray-600">
              ({sessions.reduce((s, p) => s + p.duration_minutes, 0)} min total)
            </span>
          </p>
          <div class="flex flex-col gap-1 max-h-40 overflow-y-auto">
            {#each sessions as session}
              <div class="flex items-center justify-between rounded-lg bg-gray-50 px-3 py-1.5
                          dark:bg-gray-800/60">
                <span class="text-[11px] text-gray-500 dark:text-gray-400">
                  {new Date(session.started_at).toLocaleDateString(undefined, { month: 'short', day: 'numeric' })}
                  {new Date(session.started_at).toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' })}
                </span>
                <div class="flex items-center gap-1.5">
                  <span class="font-mono text-[11px] text-gray-500 dark:text-gray-400">{session.duration_minutes}m</span>
                  <span class="h-1.5 w-1.5 rounded-full {session.was_completed ? 'bg-green-400' : 'bg-gray-300 dark:bg-gray-600'}"
                        title={session.was_completed ? 'Completed' : 'Interrupted'}></span>
                </div>
              </div>
            {/each}
          </div>
        </div>
      {/if}

      <!-- Recurrence (only in create mode) -->
      {#if !isEdit}
        <div>
          <label class="mb-1.5 block text-xs font-medium text-gray-600 dark:text-gray-400" for="task-recurrence">
            Repeat
          </label>
          <SempaSelect id="task-recurrence" bind:value={recurrenceRule}
                       placeholder="Doesn't repeat"
                       options={recurrenceOptions} />
          {#if recurrenceRule}
            <p class="mt-1.5 text-xs text-violet-600 dark:text-violet-400">↺ Creates a recurring template</p>
          {/if}
        </div>
      {/if}

      {#if error}
        <p class="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-600 dark:bg-red-950 dark:text-red-400">{error}</p>
      {/if}

    </div>

    <!-- Footer — keyboard-safe bottom padding on mobile (FIX 5) -->
    <div class="flex items-center justify-between border-t border-gray-100 px-5 py-4 dark:border-gray-800"
         style={mobile.value && !inline ? 'padding-bottom: max(16px, env(safe-area-inset-bottom, 16px));' : ''}>
      {#if isEdit && task}
        <button onclick={async () => {
                  if (!confirm('Delete this task?')) return;
                  await api.tasks.delete(task!.id);
                  onSave({ ...task!, status: 'cancelled' } as Task);
                }}
                class="text-sm text-red-500 hover:text-red-700 transition-colors dark:text-red-400 dark:hover:text-red-300">
          Delete
        </button>
      {:else}
        <span></span>
      {/if}
      <div class="flex gap-2">
        <button onclick={onClose}
                class="rounded-lg px-4 py-2 text-sm text-gray-500 hover:bg-gray-100 transition-colors
                       dark:text-gray-400 dark:hover:bg-gray-800">
          Cancel
        </button>
        <button onclick={handleSubmit} disabled={!title.trim() || saving}
                class="rounded-lg bg-blue-500 px-5 py-2 text-sm font-medium text-white
                       hover:bg-blue-600 disabled:opacity-40 disabled:cursor-not-allowed transition-colors">
          {saving ? 'Saving…' : isEdit ? 'Save changes' : recurrenceRule ? 'Create recurring' : 'Add task'}
        </button>
      </div>
    </div>
{/snippet}

{#if open}
  {#if inline}
    <div class="flex flex-col">
      {@render panelContent()}
    </div>
  {:else if mobile.value}
    <!-- Mobile bottom sheet (FIX 5) — shrinks when soft keyboard opens via visualViewport -->
    <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
    <div class="fixed inset-0 z-[89] bg-black/30 backdrop-blur-sm animate-fade-in"
         onclick={onClose}></div>
    <div role="dialog" aria-modal="true" aria-label="{isEdit ? 'Edit task' : 'New task'}"
         class="fixed bottom-0 left-0 right-0 z-[90] flex flex-col shadow-2xl"
         style="border-radius: 20px 20px 0 0; background: var(--sempa-bg-panel);
                max-height: {sheetMaxHeight}px;
                transform: translateY({dragDeltaY}px);
                transition: {draggingSheet ? 'none' : 'transform 300ms ease-out'};
                animation: sempa-sheet-up 300ms ease-out both;"
         ontouchstart={sheetTouchStart}
         ontouchmove={sheetTouchMove}
         ontouchend={sheetTouchEnd}>
      <!-- Drag handle -->
      <!-- svelte-ignore a11y_no_static_element_interactions -->
      <div class="flex justify-center pt-3 pb-1 cursor-grab shrink-0" onclick={onClose}>
        <div class="h-1 w-8 rounded-full" style="background: var(--sempa-border);"></div>
      </div>
      <div class="flex flex-1 flex-col overflow-hidden">
        {@render panelContent()}
      </div>
    </div>
  {:else}
    <!-- Desktop right-side drawer — scrim sits above the top nav (40), panel above scrim -->
    <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
    <div class="fixed inset-0 z-[49] bg-black/30 backdrop-blur-sm animate-fade-in"
         onclick={onClose}></div>
    <aside role="dialog" aria-modal="true"
           aria-label="{isEdit ? 'Edit task' : 'New task'}"
           class="fixed right-0 top-0 z-50 flex h-full w-full max-w-md flex-col shadow-2xl animate-slide-right"
           style="border-left: 1px solid var(--sempa-border); background: var(--sempa-bg-panel);">
      {@render panelContent()}
    </aside>
  {/if}
{/if}

<style>
  @keyframes sempa-sheet-up {
    from { transform: translateY(100%); }
    to   { transform: translateY(0); }
  }
</style>
