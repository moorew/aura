<script lang="ts">
  import { pomodoro } from '$lib/stores/pomodoro.svelte';

  const accentClass = $derived(
    pomodoro.phase === 'work' ? 'bg-amber-500' : 'bg-green-400'
  );

  let settingsOpen = $state(false);
  let workInput       = $state(pomodoro.workMins);
  let shortBreakInput = $state(pomodoro.shortBreakMins);
  let longBreakInput  = $state(pomodoro.longBreakMins);

  function openSettings() {
    workInput       = pomodoro.workMins;
    shortBreakInput = pomodoro.shortBreakMins;
    longBreakInput  = pomodoro.longBreakMins;
    settingsOpen    = true;
  }

  function applySettings() {
    const w  = Math.max(1, Math.min(120, workInput       || 25));
    const sb = Math.max(1, Math.min(60,  shortBreakInput || 5));
    const lb = Math.max(1, Math.min(60,  longBreakInput  || 15));
    pomodoro.setPrefs(w, sb, lb);
    settingsOpen = false;
  }
</script>

<div
  class="fixed bottom-6 right-6 z-50 w-64 rounded-2xl border border-gray-200 bg-white p-4 shadow-2xl
         dark:border-gray-700 dark:bg-gray-800"
>
  <!-- Header row -->
  <div class="mb-1 flex items-center justify-between">
    <span class="text-xs font-semibold uppercase tracking-wider text-gray-400 dark:text-gray-500">
      {pomodoro.phaseLabel}
    </span>
    <div class="flex items-center gap-1">
      <button
        onclick={openSettings}
        class="rounded p-0.5 text-gray-300 hover:text-gray-500 transition-colors"
        aria-label="Timer settings"
      >
        <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round"
            d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"/>
          <path stroke-linecap="round" stroke-linejoin="round" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"/>
        </svg>
      </button>
      <button
        onclick={() => pomodoro.stop()}
        class="rounded p-0.5 text-gray-300 hover:text-gray-500 transition-colors"
        aria-label="Stop timer"
      >
        <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12"/>
        </svg>
      </button>
    </div>
  </div>

  {#if settingsOpen}
    <!-- Settings panel -->
    <div class="mb-3 rounded-xl border border-gray-100 bg-gray-50 p-3 dark:border-gray-700 dark:bg-gray-900/60">
      <p class="mb-2 text-[10.5px] font-semibold uppercase tracking-wider text-gray-400">Durations (min)</p>
      <div class="flex flex-col gap-2">
        <label class="flex items-center justify-between text-xs text-gray-600 dark:text-gray-400">
          Focus
          <input type="number" min="1" max="120" bind:value={workInput}
                 class="w-14 rounded border border-gray-200 bg-white px-2 py-1 text-right text-xs
                        dark:border-gray-600 dark:bg-gray-800 dark:text-gray-200" />
        </label>
        <label class="flex items-center justify-between text-xs text-gray-600 dark:text-gray-400">
          Short break
          <input type="number" min="1" max="60" bind:value={shortBreakInput}
                 class="w-14 rounded border border-gray-200 bg-white px-2 py-1 text-right text-xs
                        dark:border-gray-600 dark:bg-gray-800 dark:text-gray-200" />
        </label>
        <label class="flex items-center justify-between text-xs text-gray-600 dark:text-gray-400">
          Long break
          <input type="number" min="1" max="60" bind:value={longBreakInput}
                 class="w-14 rounded border border-gray-200 bg-white px-2 py-1 text-right text-xs
                        dark:border-gray-600 dark:bg-gray-800 dark:text-gray-200" />
        </label>
      </div>
      <div class="mt-2.5 flex gap-2">
        <button onclick={applySettings}
                class="flex-1 rounded-lg bg-amber-500 py-1.5 text-xs font-medium text-white hover:bg-amber-600 transition-colors">
          Apply
        </button>
        <button onclick={() => settingsOpen = false}
                class="flex-1 rounded-lg bg-gray-100 py-1.5 text-xs text-gray-600 hover:bg-gray-200 transition-colors
                       dark:bg-gray-700 dark:text-gray-300 dark:hover:bg-gray-600">
          Cancel
        </button>
      </div>
    </div>
  {:else}
    <!-- Task title -->
    {#if pomodoro.taskTitle}
      <p class="mb-3 truncate text-sm font-medium text-gray-700 dark:text-gray-200" title={pomodoro.taskTitle}>
        {pomodoro.taskTitle}
      </p>
    {/if}

    <!-- Clock -->
    <div class="mb-3 text-center">
      <span class="font-mono text-5xl font-bold tabular-nums text-gray-900 dark:text-gray-50">
        {pomodoro.display}
      </span>
    </div>

    <!-- Progress bar -->
    <div class="mb-3 h-1.5 overflow-hidden rounded-full bg-gray-100 dark:bg-gray-700">
      <div
        class="h-full rounded-full transition-all duration-1000 {accentClass}"
        style="width: {pomodoro.progressPct}%"
      ></div>
    </div>

    <!-- Controls -->
    <button
      onclick={() => pomodoro.togglePause()}
      class="w-full rounded-lg py-2 text-sm font-medium text-white transition-colors
             {pomodoro.phase === 'work' ? 'bg-amber-500 hover:bg-amber-600' : 'bg-green-500 hover:bg-green-600'}"
    >
      {pomodoro.isRunning ? 'Pause' : 'Resume'}
    </button>

    <!-- Completed count -->
    {#if pomodoro.completedToday > 0}
      <p class="mt-2 text-center text-xs text-gray-400">
        {pomodoro.completedToday} pomodoro{pomodoro.completedToday !== 1 ? 's' : ''} today
      </p>
    {/if}
  {/if}
</div>
