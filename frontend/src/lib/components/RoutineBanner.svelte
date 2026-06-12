<script lang="ts">
  /**
   * Non-intrusive in-app prompt for the scheduled routines (weekly planning /
   * daily shutdown). Rendered at the top of the main content by +layout.svelte.
   * This is intentionally a calm banner — NOT an OS alarm.
   */
  import { routines } from '$lib/stores/routines.svelte';
  import { CalendarCheck, Moon, X } from 'lucide-svelte';

  // Weekly planning takes priority if (rarely) both are due at once.
  const mode = $derived(
    routines.weeklyPlanDue ? 'plan' : routines.shutdownDue ? 'shutdown' : null,
  );
</script>

{#if mode}
  <div class="mx-auto flex max-w-3xl items-center gap-3 rounded-xl border px-4 py-3"
       style="margin: max(40px, calc(env(safe-area-inset-top, 0px) + 12px)) 16px 0; border-color: var(--sempa-accent-bg);
              background: var(--sempa-accent-bg);">
    <div class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg"
         style="background: var(--sempa-bg-panel); color: var(--sempa-accent);">
      {#if mode === 'plan'}
        <CalendarCheck size={17} strokeWidth={2} />
      {:else}
        <Moon size={17} strokeWidth={2} />
      {/if}
    </div>

    <div class="min-w-0 flex-1">
      {#if mode === 'plan'}
        <p class="font-semibold" style="font-size: 13.5px; color: var(--sempa-text);">
          Plan your week
        </p>
        <p class="truncate" style="font-size: 11.5px; color: var(--sempa-text-soft);">
          Review your backlog and schedule what matters this week.
        </p>
      {:else}
        <p class="font-semibold" style="font-size: 13.5px; color: var(--sempa-text);">
          Daily shutdown
        </p>
        <p class="truncate" style="font-size: 11.5px; color: var(--sempa-text-soft);">
          Clear today's tasks, reschedule what's left, and close out the day.
        </p>
      {/if}
    </div>

    <button
      onclick={() => (mode === 'plan' ? routines.startWeeklyPlan() : routines.startShutdown())}
      class="shrink-0 rounded-lg px-3 py-1.5 font-semibold transition-opacity hover:opacity-90"
      style="font-size: 12.5px; background: var(--sempa-btn-bg); color: var(--sempa-btn-fg);">
      {mode === 'plan' ? 'Start planning' : 'Start shutdown'}
    </button>

    <button
      onclick={() => (mode === 'plan' ? routines.dismissWeeklyPlan() : routines.dismissShutdown())}
      aria-label="Dismiss"
      class="shrink-0 rounded-lg p-1 transition-colors"
      style="color: var(--sempa-text-dim);">
      <X size={16} />
    </button>
  </div>
{/if}
