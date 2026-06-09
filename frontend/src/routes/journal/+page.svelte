<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api';
  import { formatDate, formatWeekRange, isToday } from '$lib/utils';
  import type { DailyPlan, WeekReview } from '$lib/types';
  import SempaPattern from '$lib/components/ui/SempaPattern.svelte';

  type DailyEntry = { kind: 'daily'; date: string; plan: DailyPlan };
  type WeekEntry  = { kind: 'week'; date: string; review: WeekReview };
  type Entry = DailyEntry | WeekEntry;

  let plans   = $state<DailyPlan[]>([]);
  let reviews = $state<WeekReview[]>([]);
  let loading = $state(true);

  // Merge daily plans and week reviews into one reverse-chronological timeline.
  const entries = $derived.by<Entry[]>(() => {
    const out: Entry[] = [];
    for (const p of plans) out.push({ kind: 'daily', date: p.plan_date, plan: p });
    for (const r of reviews) out.push({ kind: 'week', date: r.week_start, review: r });
    // Week reviews sort by their week-start; sort the whole list newest first,
    // and when a week review shares a date boundary with a day, show it first.
    out.sort((a, b) => {
      if (a.date === b.date) return a.kind === 'week' ? -1 : 1;
      return a.date < b.date ? 1 : -1;
    });
    return out;
  });

  function parseList(json: string | null): string[] {
    if (!json) return [];
    try {
      const v = JSON.parse(json);
      return Array.isArray(v) ? v.filter((s) => typeof s === 'string' && s.trim()) : [];
    } catch { return []; }
  }

  async function load() {
    [plans, reviews] = await Promise.all([
      api.plans.list().catch(() => []),
      api.weeks.listReviews().catch(() => []),
    ]);
    loading = false;
  }

  onMount(load);
</script>

<div class="relative mx-auto w-full max-w-2xl px-4 py-6 sm:px-6 sm:py-10">
  <!-- Header -->
  <div class="mb-6">
    <h1 class="text-2xl font-semibold tracking-tight" style="color: var(--sempa-text);">Journal</h1>
    <p class="mt-1 text-sm" style="color: var(--sempa-text-soft);">
      Your intentions, reflections, and weekly reviews over time.
    </p>
  </div>

  {#if loading}
    <div class="flex flex-col gap-3">
      {#each Array(3) as _}
        <div class="h-24 animate-pulse rounded-2xl" style="background: var(--sempa-bg-panel);"></div>
      {/each}
    </div>
  {:else if entries.length === 0}
    <!-- Empty state -->
    <div class="relative overflow-hidden rounded-2xl py-20 text-center"
         style="border: 1px solid var(--sempa-border);">
      <div class="pointer-events-none absolute inset-0 z-0">
        <SempaPattern motif="garden" class="h-full w-full" opacity={0.9} />
      </div>
      <div class="relative z-10 flex flex-col items-center gap-2 px-6">
        <p class="text-sm font-medium" style="color: var(--sempa-text-soft);">
          Nothing here yet.
        </p>
        <p class="text-xs" style="color: var(--sempa-text-dim);">
          Set an intention when you plan a day, or jot a reflection at shutdown —
          they'll collect here.
        </p>
      </div>
    </div>
  {:else}
    <div class="flex flex-col gap-3">
      {#each entries as entry (entry.kind + entry.date)}
        {#if entry.kind === 'daily'}
          {@const intention = entry.plan.intention?.trim()}
          {@const reflection = entry.plan.reflection?.trim()}
          <a href="/day/{entry.date}"
             class="block rounded-2xl px-5 py-4 transition-opacity active:opacity-70"
             style="background: var(--sempa-bg-panel); border: 1px solid var(--sempa-border);">
            <div class="mb-2 flex items-center justify-between gap-2">
              <p class="text-xs font-semibold uppercase tracking-wider"
                 style="color: var(--sempa-text-dim);">
                {formatDate(entry.date)}{isToday(entry.date) ? ' · Today' : ''}
              </p>
            </div>
            {#if intention}
              <div class="mb-2">
                <p class="text-[10.5px] font-semibold uppercase tracking-wider"
                   style="color: var(--sempa-accent);">Intention</p>
                <p class="mt-0.5 text-sm leading-relaxed" style="color: var(--sempa-text);">{intention}</p>
              </div>
            {/if}
            {#if reflection}
              <div>
                <p class="text-[10.5px] font-semibold uppercase tracking-wider"
                   style="color: var(--sempa-text-soft);">Reflection</p>
                <p class="mt-0.5 whitespace-pre-line text-sm leading-relaxed" style="color: var(--sempa-text);">{reflection}</p>
              </div>
            {/if}
          </a>
        {:else}
          {@const wins = parseList(entry.review.wins)}
          {@const challenges = parseList(entry.review.challenges)}
          {@const nextFocus = entry.review.next_focus?.trim()}
          <a href="/week/{entry.date}/review"
             class="block rounded-2xl px-5 py-4 transition-opacity active:opacity-70"
             style="background: var(--sempa-accent-bg); border: 1px solid var(--sempa-border);">
            <div class="mb-2 flex items-center gap-2">
              <svg class="h-4 w-4 shrink-0" style="color: var(--sempa-accent);" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                <path stroke-linecap="round" d="M9 5H7a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V7a2 2 0 0 0-2-2h-2M9 5a2 2 0 0 0 2 2h2a2 2 0 0 0 2-2M9 5a2 2 0 0 1 2-2h2a2 2 0 0 1 2 2"/>
              </svg>
              <p class="text-xs font-semibold uppercase tracking-wider" style="color: var(--sempa-accent);">
                Week review · {formatWeekRange(entry.date)}
              </p>
            </div>
            {#if wins.length}
              <div class="mb-2">
                <p class="text-[10.5px] font-semibold uppercase tracking-wider" style="color: var(--sempa-text-soft);">Wins</p>
                <ul class="mt-0.5 list-disc pl-5 text-sm leading-relaxed" style="color: var(--sempa-text);">
                  {#each wins as w}<li>{w}</li>{/each}
                </ul>
              </div>
            {/if}
            {#if challenges.length}
              <div class="mb-2">
                <p class="text-[10.5px] font-semibold uppercase tracking-wider" style="color: var(--sempa-text-soft);">Challenges</p>
                <ul class="mt-0.5 list-disc pl-5 text-sm leading-relaxed" style="color: var(--sempa-text);">
                  {#each challenges as c}<li>{c}</li>{/each}
                </ul>
              </div>
            {/if}
            {#if nextFocus}
              <div>
                <p class="text-[10.5px] font-semibold uppercase tracking-wider" style="color: var(--sempa-text-soft);">Next focus</p>
                <p class="mt-0.5 whitespace-pre-line text-sm leading-relaxed" style="color: var(--sempa-text);">{nextFocus}</p>
              </div>
            {/if}
          </a>
        {/if}
      {/each}
    </div>
  {/if}
</div>
