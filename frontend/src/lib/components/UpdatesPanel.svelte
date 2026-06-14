<script lang="ts">
  /**
   * Settings → About / Updates. Shows the running version, the update channel,
   * whether automatic checks are on, when it last checked, and a manual "Check
   * for updates" button. When a newer release exists it surfaces the release
   * notes ("What's new") and a download/install action. Brand-controlled, theme
   * aware — mirrors the in-app updater design.
   */
  import { onMount } from 'svelte';
  import { updates, type UpdateChannel } from '$lib/stores/updates.svelte';
  import { RefreshCw, Download, CheckCircle2, Sparkles } from 'lucide-svelte';

  let now = $state(Date.now());
  onMount(() => {
    // A gentle, throttled check when the user lands on this page.
    void updates.check(false);
    const t = setInterval(() => (now = Date.now()), 30_000);
    return () => clearInterval(t);
  });

  function ago(iso: string | null, n: number): string {
    if (!iso) return 'never';
    const secs = Math.max(0, Math.floor((n - new Date(iso).getTime()) / 1000));
    if (secs < 60) return 'just now';
    const m = Math.floor(secs / 60);
    if (m < 60) return `${m}m ago`;
    const h = Math.floor(m / 60);
    if (h < 24) return `${h}h ago`;
    return `${Math.floor(h / 24)}d ago`;
  }

  const channels: { id: UpdateChannel; label: string }[] = [
    { id: 'stable', label: 'Stable' },
    { id: 'prerelease', label: 'Beta' },
  ];
</script>

<section class="overflow-hidden rounded-xl border" style="border-color: var(--sempa-border); background: var(--sempa-bg-panel);">
  <!-- Identity + current version -->
  <div class="flex items-center gap-3 px-5 py-4" style="border-bottom: 1px solid var(--sempa-border);">
    <span class="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl" style="background: var(--sempa-accent-bg); color: var(--sempa-accent);">
      <svg width="22" height="22" viewBox="0 0 100 100" fill="none" aria-hidden="true">
        <path d="M22,40 a28,28 0 0 0 56,0" stroke="currentColor" stroke-width="9" stroke-linecap="round" stroke-linejoin="round"/>
        <circle cx="50" cy="35" r="7.5" fill="currentColor"/>
      </svg>
    </span>
    <div class="min-w-0 flex-1">
      <p class="text-sm font-semibold" style="color: var(--sempa-text);">Sempa</p>
      <p class="text-xs" style="color: var(--sempa-text-dim);">Version {updates.current}</p>
    </div>
    {#if !updates.available && !updates.checking}
      <span class="inline-flex items-center gap-1 text-xs" style="color: var(--sempa-success);">
        <CheckCircle2 size={14} strokeWidth={2} /> Up to date
      </span>
    {/if}
  </div>

  <div class="px-5 py-4 space-y-4">
    <!-- Update available banner -->
    {#if updates.available && updates.info}
      <div class="rounded-xl p-4" style="background: var(--sempa-accent-bg);">
        <div class="flex items-center gap-2">
          <Sparkles size={16} style="color: var(--sempa-accent);" />
          <p class="text-sm font-semibold" style="color: var(--sempa-accent);">
            Update available — {updates.info.version}
          </p>
        </div>
        {#if updates.info.notes}
          <div class="mt-2 max-h-44 overflow-y-auto whitespace-pre-wrap text-xs leading-relaxed" style="color: var(--sempa-text-soft);">{updates.info.notes}</div>
        {/if}
        <div class="mt-3 flex flex-wrap items-center gap-2">
          <a href={updates.info.downloadUrl} target="_blank" rel="noopener noreferrer"
             class="inline-flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs font-semibold text-white"
             style="background: var(--sempa-accent);">
            <Download size={14} strokeWidth={2} /> Download update
          </a>
          <a href={updates.info.url} target="_blank" rel="noopener noreferrer"
             class="rounded-lg px-3 py-1.5 text-xs font-medium"
             style="border: 1px solid var(--sempa-border); color: var(--sempa-text-soft);">
            What’s new
          </a>
        </div>
      </div>
    {/if}

    <!-- Channel -->
    <div class="flex items-center justify-between gap-4">
      <div class="min-w-0">
        <p class="text-xs font-medium" style="color: var(--sempa-text-soft);">Update channel</p>
        <p class="mt-0.5 text-[11px]" style="color: var(--sempa-text-dim);">Beta surfaces pre-releases earlier.</p>
      </div>
      <div style="display:flex; border-radius:9999px; border:1px solid var(--sempa-border); padding:3px; gap:2px;">
        {#each channels as c}
          <button onclick={() => updates.setChannel(c.id)}
                  class="transition-colors"
                  style="border-radius:9999px; padding:5px 14px; font-size:12px; border:none; cursor:pointer;
                         {updates.channel === c.id
                           ? 'background: var(--sempa-accent-bg); color: var(--sempa-accent); font-weight:600;'
                           : 'background: transparent; color: var(--sempa-text-soft);'}">
            {c.label}
          </button>
        {/each}
      </div>
    </div>

    <!-- Automatic updates toggle -->
    <div class="flex items-center justify-between gap-4">
      <div class="min-w-0">
        <p class="text-xs font-medium" style="color: var(--sempa-text-soft);">Automatic checks</p>
        <p class="mt-0.5 text-[11px]" style="color: var(--sempa-text-dim);">Check for new versions in the background.</p>
      </div>
      <button onclick={() => updates.setAutoCheck(!updates.autoCheck)}
              role="switch" aria-checked={updates.autoCheck} aria-label="Automatic update checks"
              class="relative shrink-0 rounded-full transition-colors"
              style="width:44px; height:24px; padding:0; border:none; cursor:pointer;
                     background: {updates.autoCheck ? 'var(--sempa-accent)' : 'var(--sempa-border)'};">
        <span class="absolute rounded-full bg-white" style="top:4px; left:{updates.autoCheck ? '24px' : '4px'}; width:16px; height:16px; transition: left 150ms ease;"></span>
      </button>
    </div>

    <!-- Last checked + manual check -->
    <div class="flex items-center justify-between gap-4" style="border-top: 1px solid var(--sempa-border); padding-top: 16px;">
      <p class="text-[11px]" style="color: var(--sempa-text-dim);">
        Last checked {ago(updates.lastChecked, now)}
        {#if updates.error}· <span style="color: var(--sempa-amber);">{updates.error}</span>{/if}
      </p>
      <button onclick={() => updates.check(true)} disabled={updates.checking}
              class="inline-flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs font-medium transition-colors disabled:opacity-50"
              style="border: 1px solid var(--sempa-border); color: var(--sempa-text);">
        <span class:spin={updates.checking}><RefreshCw size={13} strokeWidth={2} /></span>
        {updates.checking ? 'Checking…' : 'Check for updates'}
      </button>
    </div>
  </div>
</section>

<style>
  .spin { display: inline-flex; animation: upd-spin 1s linear infinite; }
  @keyframes upd-spin { to { transform: rotate(360deg); } }
</style>
