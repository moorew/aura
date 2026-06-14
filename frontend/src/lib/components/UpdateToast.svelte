<script lang="ts">
  /**
   * Brand-controlled "update available" toast — the in-app prompt from the
   * updater design. Slides in at the bottom-right when a newer release is
   * available and not yet dismissed; offers Download + What's new, or Later.
   * Purely presentational on top of the `updates` store.
   */
  import { updates } from '$lib/stores/updates.svelte';
  import { Download, X, Sparkles } from 'lucide-svelte';

  let expanded = $state(false);
</script>

{#if updates.available && updates.info}
  <div class="fixed bottom-5 right-5 z-[70] w-80 max-w-[calc(100vw-2.5rem)] overflow-hidden rounded-2xl shadow-2xl"
       style="background: var(--sempa-bg-panel); border: 1px solid var(--sempa-border);"
       role="status" aria-live="polite">
    <div class="flex items-start gap-3 p-4">
      <span class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg" style="background: var(--sempa-accent-bg); color: var(--sempa-accent);">
        <Sparkles size={16} strokeWidth={2} />
      </span>
      <div class="min-w-0 flex-1">
        <p class="text-sm font-semibold" style="color: var(--sempa-text);">Update available</p>
        <p class="mt-0.5 text-xs" style="color: var(--sempa-text-dim);">
          Sempa {updates.info.version} is ready to install.
        </p>

        {#if expanded && updates.info.notes}
          <div class="mt-2 max-h-32 overflow-y-auto whitespace-pre-wrap rounded-lg p-2 text-[11px] leading-relaxed"
               style="background: var(--sempa-bg-main); color: var(--sempa-text-soft);">{updates.info.notes}</div>
        {/if}

        <div class="mt-3 flex items-center gap-2">
          <a href={updates.info.downloadUrl} target="_blank" rel="noopener noreferrer"
             class="inline-flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs font-semibold text-white"
             style="background: var(--sempa-accent);">
            <Download size={13} strokeWidth={2} /> Download
          </a>
          {#if updates.info.notes}
            <button onclick={() => (expanded = !expanded)}
                    class="rounded-lg px-2.5 py-1.5 text-xs font-medium transition-colors"
                    style="color: var(--sempa-text-soft);">
              {expanded ? 'Hide notes' : 'What’s new'}
            </button>
          {/if}
          <button onclick={() => updates.dismiss()}
                  class="ml-auto rounded-lg px-2.5 py-1.5 text-xs font-medium transition-colors"
                  style="color: var(--sempa-text-dim);">
            Later
          </button>
        </div>
      </div>
      <button onclick={() => updates.dismiss()} aria-label="Dismiss update notification"
              class="-mr-1 -mt-1 rounded-lg p-1 transition-colors" style="color: var(--sempa-text-dim);">
        <X size={15} strokeWidth={2} />
      </button>
    </div>
  </div>
{/if}
