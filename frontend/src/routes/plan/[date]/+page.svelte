<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { page } from '$app/stores';
  import { api } from '$lib/api';
  import type { Task } from '$lib/types';
  import { formatDate, formatMinutes, isToday, offsetDate, weekStart } from '$lib/utils';
  import SempaPattern from '$lib/components/ui/SempaPattern.svelte';

  let date       = $derived($page.params.date ?? new Date().toISOString().split('T')[0]);
  let yesterday  = $derived(offsetDate(date, -1));

  let step                  = $state<1 | 2 | 3>(1);
  let intention             = $state('');
  let carryover             = $state<Task[]>([]);
  let todayTasks            = $state<Task[]>([]);
  let selected              = $state(new Set<string>());
  let saving                = $state(false);
  let error                 = $state<string | null>(null);
  let intentionEl: HTMLTextAreaElement | undefined = $state();
  let yesterdayReflection   = $state<string | null>(null);

  let totalEstimate = $derived(
    todayTasks.reduce((sum, t) => sum + (t.time_estimate_minutes ?? 0), 0)
  );

  onMount(async () => {
    try {
      // Load existing plan if any + yesterday's reflection
      try {
        const plan = await api.plans.get(date);
        if (plan.intention) intention = plan.intention;
      } catch { /* 404 is expected */ }
      try {
        const yPlan = await api.plans.get(yesterday);
        if (yPlan.reflection) yesterdayReflection = yPlan.reflection;
      } catch { /* ignore */ }

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

<div class="flex min-h-full flex-col items-center justify-center px-4 py-12 animate-fade-in">
  <!-- Step indicators -->
  <div class="mb-10 flex items-center gap-2">
    {#each stepLabels as label, i}
      <div class="flex items-center gap-2">
        <div class="flex h-6 w-6 items-center justify-center rounded-full text-xs font-semibold"
             style={i + 1 <= step
               ? 'background: var(--sempa-accent); color: var(--sempa-btn-fg);'
               : 'background: var(--sempa-bg-panel); border: 1px solid var(--sempa-border); color: var(--sempa-text-dim);'}>
          {#if i + 1 < step}
            <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="2.5" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/>
            </svg>
          {:else}
            {i + 1}
          {/if}
        </div>
        <span class="text-xs {i + 1 === step ? 'font-medium' : ''}"
              style="color: {i + 1 === step ? 'var(--sempa-text)' : 'var(--sempa-text-dim)'};">{label}</span>
        {#if i < stepLabels.length - 1}
          <div class="h-px w-8" style="background: var(--sempa-border);"></div>
        {/if}
      </div>
    {/each}
  </div>

  <div class="w-full max-w-md">
    <!-- ── Step 1: Intention ───────────────────────────────────────────── -->
    {#if step === 1}
      {#key step}
      <div class="animate-fade-in relative overflow-hidden"
           style="border-radius:16px; border: 1px solid var(--sempa-border);
                  background: var(--sempa-bg-panel); padding:28px 28px 24px;">
        <div class="absolute top-0 right-0 w-52 h-52 pointer-events-none z-0" style="transform: rotate(180deg);">
          <SempaPattern motif="aurora" class="w-full h-full" opacity={0.7} />
        </div>
        <div class="relative z-10">
        <p class="mb-1 text-2xl">🌅</p>
        <h1 class="mb-1 text-xl font-semibold" style="color: var(--sempa-text);">
          {isToday(date) ? 'Good morning!' : formatDate(date)}
        </h1>
        <p class="mb-6 text-sm" style="color: var(--sempa-text-soft);">What's your theme or focus for today?</p>

        {#if yesterdayReflection}
          <div class="mb-5 rounded-xl px-4 py-3" style="background: var(--sempa-bg-main); border: 1px solid var(--sempa-border);">
            <p class="text-[10px] font-semibold uppercase tracking-wider mb-1" style="color: var(--sempa-text-dim);">Yesterday you noted</p>
            <p class="text-sm italic" style="color: var(--sempa-text-soft);">"{yesterdayReflection}"</p>
          </div>
        {/if}

        <textarea
          bind:this={intentionEl}
          bind:value={intention}
          onkeydown={(e) => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); goToStep2(); } }}
          onfocus={(e) => (e.currentTarget as HTMLElement).style.borderColor = 'var(--sempa-accent)'}
          onblur={(e) => (e.currentTarget as HTMLElement).style.borderColor = 'var(--sempa-border)'}
          rows="3"
          placeholder="e.g. 'Ship the auth feature and prep for standup'"
          style="border: 1px solid var(--sempa-border); background: var(--sempa-bg-main);
                 border-radius:12px; padding:16px; resize:none; width:100%; min-height:100px;
                 font-size:15px; color:var(--sempa-text); line-height:1.6; outline:none;"
        ></textarea>

        <div class="mt-4 flex justify-end">
          <button
            onclick={goToStep2}
            style="background: var(--sempa-btn-bg); color: var(--sempa-btn-fg); border-radius:12px;
                   padding:10px 20px; font-size:14px; font-weight:500; border:none; cursor:pointer;
                   display:inline-flex; align-items:center; gap:8px;"
            onmouseenter={(e) => (e.currentTarget as HTMLElement).style.opacity = '0.88'}
            onmouseleave={(e) => (e.currentTarget as HTMLElement).style.opacity = '1'}
          >
            Continue
            <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7"/>
            </svg>
          </button>
        </div>
        </div><!-- end z-10 -->
      </div>
      {/key}

    <!-- ── Step 2: Carryover ───────────────────────────────────────────── -->
    {:else if step === 2}
      {#key step}
      <div class="animate-fade-in relative overflow-hidden"
           style="border-radius:16px; border: 1px solid var(--sempa-border);
                  background: var(--sempa-bg-panel); padding:28px 28px 24px;">
        <div class="absolute top-0 right-0 w-52 h-52 pointer-events-none z-0" style="transform: rotate(180deg);">
          <SempaPattern motif="aurora" class="w-full h-full" opacity={0.7} />
        </div>
        <div class="relative z-10">
        <p class="mb-1 text-2xl">↩️</p>
        <h1 class="mb-1 text-xl font-semibold" style="color: var(--sempa-text);">From yesterday</h1>
        <p class="mb-6 text-sm" style="color: var(--sempa-text-soft);">
          {carryover.length === 0
            ? "You cleared your board yesterday. Nicely done."
            : `${carryover.length} task${carryover.length !== 1 ? 's' : ''} didn't get done. Move them to today?`}
        </p>

        {#if carryover.length > 0}
          <div class="flex flex-col mb-6">
            {#each carryover as task (task.id)}
              {@const isSel = selected.has(task.id)}
              <div class="flex items-center gap-3 transition-colors"
                   style="display:flex; align-items:center; gap:12px; padding:10px 14px;
                          border-radius:10px; margin-bottom:6px;
                          {isSel
                            ? 'border: 1px solid color-mix(in srgb, var(--sempa-accent) 40%, transparent); background: color-mix(in srgb, var(--sempa-accent) 8%, transparent);'
                            : 'border: 1px solid var(--sempa-border); background: var(--sempa-bg-panel);'}">
                <button type="button" role="checkbox" aria-checked={isSel}
                        onclick={() => toggleSelect(task.id)}
                        class="mt-0.5 h-5 w-5 shrink-0 rounded-md border-2 flex items-center justify-center transition-colors"
                        style={isSel
                          ? 'border-color: var(--sempa-accent); background: var(--sempa-accent);'
                          : 'border-color: var(--sempa-text-dim);'}
                        onmouseenter={(e) => { if (!isSel) (e.currentTarget as HTMLElement).style.borderColor = 'var(--sempa-accent)'; }}
                        onmouseleave={(e) => { if (!isSel) (e.currentTarget as HTMLElement).style.borderColor = 'var(--sempa-text-dim)'; }}>
                  {#if isSel}
                    <svg class="h-3 w-3" style="color: var(--sempa-btn-fg);" fill="none" stroke="currentColor" stroke-width="3" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/>
                    </svg>
                  {/if}
                </button>
                <div>
                  <p class="text-sm" style="color: var(--sempa-text);">{task.title}</p>
                  {#if task.time_estimate_minutes}
                    <span style="background: var(--sempa-accent-bg); color: var(--sempa-accent);
                                 border-radius:9999px; padding:3px 10px; font-size:12px; font-weight:500;
                                 display:inline-block; margin-top:4px;">
                      {formatMinutes(task.time_estimate_minutes)}
                    </span>
                  {/if}
                </div>
              </div>
            {/each}
          </div>
        {/if}

        {#if error}<p class="mb-4 text-xs text-red-500">{error}</p>{/if}

        <div class="flex justify-between">
          <button onclick={() => step = 1} class="px-4 py-2 text-sm transition-colors" style="color: var(--sempa-text-dim);">
            ← Back
          </button>
          <button
            onclick={goToStep3}
            disabled={saving}
            class="disabled:opacity-50"
            style="background: var(--sempa-btn-bg); color: var(--sempa-btn-fg); border-radius:12px;
                   padding:10px 20px; font-size:14px; font-weight:500; border:none; cursor:pointer;
                   display:inline-flex; align-items:center; gap:8px;"
            onmouseenter={(e) => (e.currentTarget as HTMLElement).style.opacity = '0.88'}
            onmouseleave={(e) => (e.currentTarget as HTMLElement).style.opacity = '1'}
          >
            {saving ? 'Moving…' : 'Continue'}
            <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7"/>
            </svg>
          </button>
        </div>
        </div><!-- end z-10 -->
      </div>
      {/key}

    <!-- ── Step 3: Ready ───────────────────────────────────────────────── -->
    {:else}
      {#key step}
      <div class="animate-fade-in relative overflow-hidden"
           style="border-radius:16px; border: 1px solid var(--sempa-border);
                  background: var(--sempa-bg-panel); padding:28px 28px 24px;">
        <div class="absolute top-0 right-0 w-52 h-52 pointer-events-none z-0" style="transform: rotate(180deg);">
          <SempaPattern motif="aurora" class="w-full h-full" opacity={0.7} />
        </div>
        <div class="relative z-10">
        <p class="mb-1 text-2xl">✅</p>
        <h1 class="mb-1 text-xl font-semibold" style="color: var(--sempa-text);">You're set for today!</h1>

        {#if intention}
          <div class="mb-4 rounded-xl px-4 py-3" style="background: var(--sempa-accent-bg);">
            <p class="text-xs font-medium mb-0.5" style="color: var(--sempa-accent);">Today's intention</p>
            <p class="text-sm" style="color: var(--sempa-text);">{intention}</p>
          </div>
        {/if}

        <p class="mb-1 text-sm" style="color: var(--sempa-text-soft);">
          {todayTasks.filter(t => t.status !== 'done').length} tasks on your board
          {#if totalEstimate > 0}· {formatMinutes(totalEstimate)} estimated{/if}
        </p>

        {#if error}<p class="mb-4 text-xs text-red-500">{error}</p>{/if}

        <div class="mt-6 flex justify-between">
          <button onclick={() => step = 2} class="px-4 py-2 text-sm transition-colors" style="color: var(--sempa-text-dim);">
            ← Back
          </button>
          <button
            onclick={startFocusing}
            disabled={saving}
            class="disabled:opacity-50"
            style="background: var(--sempa-btn-bg); color: var(--sempa-btn-fg); border-radius:12px;
                   padding:10px 20px; font-size:14px; font-weight:500; border:none; cursor:pointer;
                   display:inline-flex; align-items:center; gap:8px;"
            onmouseenter={(e) => (e.currentTarget as HTMLElement).style.opacity = '0.88'}
            onmouseleave={(e) => (e.currentTarget as HTMLElement).style.opacity = '1'}
          >
            {saving ? 'Saving…' : 'Start focusing'}
            <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" d="M13 9l3 3m0 0l-3 3m3-3H8m13 0a9 9 0 11-18 0 9 9 0 0118 0z"/>
            </svg>
          </button>
        </div>
        </div><!-- end z-10 -->
      </div>
      {/key}
    {/if}
  </div>
</div>
