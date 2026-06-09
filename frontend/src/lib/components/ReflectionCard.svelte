<script lang="ts">
  /**
   * Contextual display of a day's intention and/or reflection, shown inline on
   * the day view. Gated by the caller via prefs.contextualReflections. When a
   * field is empty and it's the relevant day, shows a gentle prompt that links
   * to the Plan Day / Shutdown ritual so the comments are actually reachable.
   */
  import { ClipboardCheck, Moon } from 'lucide-svelte';

  let {
    date,
    intention = null,
    reflection = null,
    show = 'both',
    promptWhenEmpty = false,
  }: {
    date: string;
    intention?: string | null;
    reflection?: string | null;
    show?: 'intention' | 'reflection' | 'both';
    /** Show a "set an intention / add a reflection" link when the field is empty. */
    promptWhenEmpty?: boolean;
  } = $props();

  const showIntention = $derived(show === 'both' || show === 'intention');
  const showReflection = $derived(show === 'both' || show === 'reflection');

  const hasIntention = $derived(!!intention?.trim());
  const hasReflection = $derived(!!reflection?.trim());

  // Only render at all if there's something to show or a prompt to offer.
  const visible = $derived(
    (showIntention && (hasIntention || promptWhenEmpty)) ||
    (showReflection && (hasReflection || promptWhenEmpty))
  );
</script>

{#if visible}
  <div class="flex flex-col gap-2">
    {#if showIntention}
      {#if hasIntention}
        <div class="rounded-xl px-4 py-3" style="background: var(--sempa-accent-bg); border: 1px solid var(--sempa-border);">
          <p class="text-[10.5px] font-semibold uppercase tracking-wider" style="color: var(--sempa-accent);">Today's intention</p>
          <p class="mt-1 text-sm leading-relaxed" style="color: var(--sempa-text);">{intention}</p>
        </div>
      {:else if promptWhenEmpty}
        <a href="/plan/{date}"
           class="flex items-center gap-2 rounded-xl px-4 py-2.5 text-sm transition-opacity active:opacity-70"
           style="border: 1px dashed var(--sempa-border); color: var(--sempa-text-soft);">
          <ClipboardCheck size={15} strokeWidth={1.75} />
          Set an intention for the day
        </a>
      {/if}
    {/if}

    {#if showReflection}
      {#if hasReflection}
        <div class="rounded-xl px-4 py-3" style="background: var(--sempa-bg-panel); border: 1px solid var(--sempa-border);">
          <p class="text-[10.5px] font-semibold uppercase tracking-wider" style="color: var(--sempa-text-soft);">Reflection</p>
          <p class="mt-1 whitespace-pre-line text-sm leading-relaxed" style="color: var(--sempa-text);">{reflection}</p>
        </div>
      {:else if promptWhenEmpty}
        <a href="/shutdown/{date}"
           class="flex items-center gap-2 rounded-xl px-4 py-2.5 text-sm transition-opacity active:opacity-70"
           style="border: 1px dashed var(--sempa-border); color: var(--sempa-text-soft);">
          <Moon size={15} strokeWidth={1.75} />
          Add an end-of-day reflection
        </a>
      {/if}
    {/if}
  </div>
{/if}
