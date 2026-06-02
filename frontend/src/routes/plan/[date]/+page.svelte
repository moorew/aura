<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { page } from '$app/stores';
  import { api } from '$lib/api';
  import type { Task } from '$lib/types';
  import { formatDate, formatMinutes, isToday, offsetDate, weekStart } from '$lib/utils';

  let date       = $derived($page.params.date ?? new Date().toISOString().split('T')[0]);
  let yesterday  = $derived(offsetDate(date, -1));

  let step         = $state<1 | 2 | 3>(1);
  let intention    = $state('');
  let carryover    = $state<Task[]>([]);
  let todayTasks   = $state<Task[]>([]);
  let selected     = $state(new Set<string>());
  let saving       = $state(false);
  let error        = $state<string | null>(null);
  let intentionEl: HTMLTextAreaElement | undefined = $state();

  let totalEstimate = $derived(
    todayTasks.reduce((sum, t) => sum + (t.time_estimate_minutes ?? 0), 0)
  );

  onMount(async () => {
    try {
      // Load existing plan if any
      try {
        const plan = await api.plans.get(date);
        if (plan.intention) intention = plan.intention;
      } catch { /* 404 is expected */ }

      const [yTasks, dTasks] = await Promise.all([
        api.tasks.listByDate(yesterday),
        api.tasks.listByDate(date),
      ]);

      carryover  = yTasks.filter(t => t.status !== 'done' && t.status !== 'cancelled');
      todayTasks = dTasks;
      selected   = new Set(carryover.map(t => t.id)); // pre-select all
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to load';
    }

    setTimeout(() => intentionEl?.focus(), 100);
  });

  function toggleSelect(id: string) {
    const s = new Set(selected);
    s.has(id) ? s.delete(id) : s.add(id);
    selected = s;
  }

  async function goToStep2() {
    step = 2;
    if (carryover.length === 0) await goToStep3();
  }

  async function goToStep3() {
    // Move selected carryover tasks to today
    saving = true;
    try {
      const moves = [...selected].map(id =>
        api.tasks.update(id, { planned_date: date, week_start: weekStart(date), status: 'planned' })
      );
      const moved = await Promise.all(moves);
      // Refresh today's tasks
      todayTasks = await api.tasks.listByDate(date);
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to move tasks';
    } finally {
      saving = false;
    }
    step = 3;
  }

  async function startFocusing() {
    saving = true;
    try {
      await api.plans.upsert(date, {
        status: 'active',
        intention: intention.trim() || null,
      });
      goto(`/day/${date}`);
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to save plan';
      saving = false;
    }
  }

  const stepLabels = ['Intention', 'Carryover', 'Ready'];
</script>

<svelte:head><title>Plan {isToday(date) ? 'Today' : date} — Sempa</title></svelte:head>

<div class="flex min-h-full flex-col items-center justify-center px-4 py-12">
  <!-- Step indicators -->
  <div class="mb-10 flex items-center gap-2">
    {#each stepLabels as label, i}
      <div class="flex items-center gap-2">
        <div class="flex h-6 w-6 items-center justify-center rounded-full text-xs font-semibold
                    {i + 1 <= step ? 'bg-blue-500 text-white' : 'bg-gray-100 text-gray-400'}">
          {#if i + 1 < step}
            <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="2.5" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/>
            </svg>
          {:else}
            {i + 1}
          {/if}
        </div>
        <span class="text-xs {i + 1 === step ? 'font-medium text-gray-700' : 'text-gray-400'}">{label}</span>
        {#if i < stepLabels.length - 1}
          <div class="h-px w-8 bg-gray-200"></div>
        {/if}
      </div>
    {/each}
  </div>

  <div class="w-full max-w-md">
    <!-- ── Step 1: Intention ───────────────────────────────────────────── -->
    {#if step === 1}
      <div class="rounded-2xl border border-gray-200 bg-white p-8 shadow-sm">
        <p class="mb-1 text-2xl">🌅</p>
        <h1 class="mb-1 text-xl font-semibold text-gray-900">
          {isToday(date) ? 'Good morning!' : formatDate(date)}
        </h1>
        <p class="mb-6 text-sm text-gray-500">What's your theme or focus for today?</p>

        <textarea
          bind:this={intentionEl}
          bind:value={intention}
          onkeydown={(e) => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); goToStep2(); } }}
          rows="3"
          placeholder="e.g. 'Ship the auth feature and prep for standup'"
          class="w-full resize-none rounded-xl border border-gray-200 bg-gray-50 px-4 py-3 text-sm
                 text-gray-800 placeholder-gray-400 outline-none focus:border-blue-400 focus:bg-white
                 focus:ring-2 focus:ring-blue-100"
        ></textarea>

        <div class="mt-4 flex justify-end">
          <button
            onclick={goToStep2}
            class="flex items-center gap-1.5 rounded-xl bg-blue-500 px-5 py-2.5 text-sm font-medium
                   text-white hover:bg-blue-600 transition-colors"
          >
            Continue
            <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7"/>
            </svg>
          </button>
        </div>
      </div>

    <!-- ── Step 2: Carryover ───────────────────────────────────────────── -->
    {:else if step === 2}
      <div class="rounded-2xl border border-gray-200 bg-white p-8 shadow-sm">
        <p class="mb-1 text-2xl">↩️</p>
        <h1 class="mb-1 text-xl font-semibold text-gray-900">From yesterday</h1>
        <p class="mb-6 text-sm text-gray-500">
          {carryover.length === 0
            ? "You cleared your board yesterday. Nicely done."
            : `${carryover.length} task${carryover.length !== 1 ? 's' : ''} didn't get done. Move them to today?`}
        </p>

        {#if carryover.length > 0}
          <div class="flex flex-col gap-2 mb-6">
            {#each carryover as task (task.id)}
              <label class="flex cursor-pointer items-start gap-3 rounded-lg border px-3 py-2.5 transition-colors
                            {selected.has(task.id) ? 'border-blue-200 bg-blue-50' : 'border-gray-100 hover:border-gray-200'}">
                <input
                  type="checkbox"
                  checked={selected.has(task.id)}
                  onchange={() => toggleSelect(task.id)}
                  class="mt-0.5 h-4 w-4 shrink-0 accent-blue-500"
                />
                <div>
                  <p class="text-sm text-gray-700">{task.title}</p>
                  {#if task.time_estimate_minutes}
                    <p class="text-xs text-gray-400">{formatMinutes(task.time_estimate_minutes)}</p>
                  {/if}
                </div>
              </label>
            {/each}
          </div>
        {/if}

        {#if error}<p class="mb-4 text-xs text-red-500">{error}</p>{/if}

        <div class="flex justify-between">
          <button onclick={() => step = 1} class="px-4 py-2 text-sm text-gray-400 hover:text-gray-600 transition-colors">
            ← Back
          </button>
          <button
            onclick={goToStep3}
            disabled={saving}
            class="flex items-center gap-1.5 rounded-xl bg-blue-500 px-5 py-2.5 text-sm font-medium
                   text-white hover:bg-blue-600 disabled:opacity-50 transition-colors"
          >
            {saving ? 'Moving…' : 'Continue'}
            <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7"/>
            </svg>
          </button>
        </div>
      </div>

    <!-- ── Step 3: Ready ───────────────────────────────────────────────── -->
    {:else}
      <div class="rounded-2xl border border-gray-200 bg-white p-8 shadow-sm">
        <p class="mb-1 text-2xl">✅</p>
        <h1 class="mb-1 text-xl font-semibold text-gray-900">You're set for today!</h1>

        {#if intention}
          <div class="mb-4 rounded-xl bg-blue-50 px-4 py-3">
            <p class="text-xs font-medium text-blue-500 mb-0.5">Today's intention</p>
            <p class="text-sm text-blue-800">{intention}</p>
          </div>
        {/if}

        <p class="mb-1 text-sm text-gray-500">
          {todayTasks.filter(t => t.status !== 'done').length} tasks on your board
          {#if totalEstimate > 0}· {formatMinutes(totalEstimate)} estimated{/if}
        </p>

        {#if error}<p class="mb-4 text-xs text-red-500">{error}</p>{/if}

        <div class="mt-6 flex justify-between">
          <button onclick={() => step = 2} class="px-4 py-2 text-sm text-gray-400 hover:text-gray-600 transition-colors">
            ← Back
          </button>
          <button
            onclick={startFocusing}
            disabled={saving}
            class="flex items-center gap-2 rounded-xl bg-blue-500 px-6 py-2.5 text-sm font-medium
                   text-white hover:bg-blue-600 disabled:opacity-50 transition-colors"
          >
            {saving ? 'Saving…' : 'Start focusing'}
            <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" d="M13 9l3 3m0 0l-3 3m3-3H8m13 0a9 9 0 11-18 0 9 9 0 0118 0z"/>
            </svg>
          </button>
        </div>
      </div>
    {/if}
  </div>
</div>
