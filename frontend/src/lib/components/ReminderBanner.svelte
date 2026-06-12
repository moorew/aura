<script lang="ts">
  /**
   * In-app banner(s) for task reminders that have just come due while the app is
   * open. Rendered at the top of the main content by +layout.svelte, above the
   * routine banner. Unlike the routine prompts this IS an alarm surface — it
   * tells the user exactly which task rang, with quick Open / Done / Snooze
   * actions. It's the cross-platform backstop for when a native OS toast is
   * suppressed (e.g. Windows focus assist) and only the sound was heard.
   */
  import { goto } from '$app/navigation';
  import { reminderAlerts } from '$lib/stores/reminderAlerts.svelte';
  import { isTauri } from '$lib/platform';
  import { Bell, X } from 'lucide-svelte';

  // On desktop the same alerts surface as a floating top-right card outside the
  // app window (see $lib/desktopReminderPopup), so suppress the in-app banner
  // there to avoid showing the reminder twice. Web + Android keep the banner.
  const show = $derived(reminderAlerts.alerts.length > 0 && !isTauri());

  function open(taskId: string) {
    reminderAlerts.dismiss(taskId);
    goto(`/focus/${taskId}`);
  }
</script>

{#if show}
  <div class="mx-auto flex max-w-3xl flex-col gap-2"
       style="margin: calc(env(safe-area-inset-top, 0px) + 12px) 16px 0;">
    {#each reminderAlerts.alerts as a (a.taskId)}
      <div class="flex items-center gap-3 rounded-xl border px-4 py-3"
           style="border-color: var(--sempa-accent); background: var(--sempa-accent-bg);">
        <div class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg"
             style="background: var(--sempa-bg-panel); color: var(--sempa-accent);">
          <Bell size={17} strokeWidth={2} />
        </div>

        <div class="min-w-0 flex-1">
          <p class="font-semibold" style="font-size: 11px; letter-spacing: 0.04em; text-transform: uppercase; color: var(--sempa-accent);">
            Reminder
          </p>
          <p class="truncate" style="font-size: 13.5px; color: var(--sempa-text);">{a.title}</p>
        </div>

        <button
          onclick={() => open(a.taskId)}
          class="shrink-0 rounded-lg px-3 py-1.5 font-semibold transition-opacity hover:opacity-90"
          style="font-size: 12.5px; background: var(--sempa-btn-bg); color: var(--sempa-btn-fg);">
          Open
        </button>
        <button
          onclick={() => reminderAlerts.markDone(a.taskId)}
          class="shrink-0 rounded-lg px-2.5 py-1.5 font-medium transition-colors"
          style="font-size: 12.5px; color: var(--sempa-text-soft); border: 1px solid var(--sempa-border);">
          Done
        </button>
        <button
          onclick={() => reminderAlerts.snooze(a.taskId)}
          class="shrink-0 rounded-lg px-2.5 py-1.5 font-medium transition-colors"
          style="font-size: 12.5px; color: var(--sempa-text-soft); border: 1px solid var(--sempa-border);"
          title="Snooze 1 hour">
          Snooze
        </button>

        <button
          onclick={() => reminderAlerts.dismiss(a.taskId)}
          aria-label="Dismiss"
          class="shrink-0 rounded-lg p-1 transition-colors"
          style="color: var(--sempa-text-dim);">
          <X size={16} />
        </button>
      </div>
    {/each}
  </div>
{/if}
