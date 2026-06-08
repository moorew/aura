<script lang="ts">
  /**
   * Compact sync-status pill bound to the shared syncStore. Shows whether the
   * device is online, currently syncing, or has local changes still queued, and
   * lets the user kick off a sync by clicking. Only meaningful on local-first
   * platforms (desktop/Android); renders nothing on plain web.
   */
  import { syncStore, sync as runSync } from '$lib/sync';
  import { hasLocalDb } from '$lib/tauri/bridge';
  import { onMount } from 'svelte';
  import { RefreshCw, Cloud, CloudOff, CloudUpload } from 'lucide-svelte';

  // A ticking clock so the relative "synced Xm ago" label stays current even
  // when nothing else changes. Updated every 30s.
  let nowMs = $state(Date.now());
  onMount(() => {
    const t = setInterval(() => { nowMs = Date.now(); }, 30_000);
    return () => clearInterval(t);
  });

  // Relative "last synced" label, recomputed when lastSyncedAt or the tick changes.
  function ago(iso: string | null, now: number): string {
    if (!iso) return '';
    const secs = Math.max(0, Math.floor((now - new Date(iso).getTime()) / 1000));
    if (secs < 60) return 'just now';
    const mins = Math.floor(secs / 60);
    if (mins < 60) return `${mins}m ago`;
    const hrs = Math.floor(mins / 60);
    if (hrs < 24) return `${hrs}h ago`;
    return `${Math.floor(hrs / 24)}d ago`;
  }

  const syncState = $derived(
    syncStore.syncing ? 'syncing'
    : !syncStore.online ? 'offline'
    : syncStore.pending > 0 ? 'pending'
    : 'synced',
  );

  const label = $derived(
    syncState === 'syncing' ? 'Syncing…'
    : syncState === 'offline' ? (syncStore.pending > 0 ? `Offline · ${syncStore.pending} pending` : 'Offline')
    : syncState === 'pending' ? `${syncStore.pending} to sync`
    : syncStore.lastSyncedAt ? `Synced ${ago(syncStore.lastSyncedAt, nowMs)}` : 'Synced',
  );

  const color = $derived(
    syncState === 'offline' ? 'var(--sempa-text-dim)'
    : syncState === 'pending' ? 'var(--sempa-accent)'
    : 'var(--sempa-text-soft)',
  );
</script>

{#if hasLocalDb()}
  <button
    onclick={() => runSync()}
    disabled={syncStore.syncing}
    title={syncState === 'offline'
      ? 'No connection to your sempa server. Changes are saved locally and will sync when you reconnect.'
      : 'Click to sync now'}
    class="flex w-full items-center gap-2 rounded-lg px-3 py-1.5 text-[11px] transition-colors"
    style="color: {color}; background: transparent;"
    onmouseenter={(e) => ((e.currentTarget as HTMLElement).style.background = 'var(--sempa-bg-hover, rgba(0,0,0,0.05))')}
    onmouseleave={(e) => ((e.currentTarget as HTMLElement).style.background = 'transparent')}>
    <span class="flex-shrink-0" class:spin={syncStore.syncing}>
      {#if syncState === 'syncing'}
        <RefreshCw size={13} strokeWidth={1.75} />
      {:else if syncState === 'offline'}
        <CloudOff size={13} strokeWidth={1.75} />
      {:else if syncState === 'pending'}
        <CloudUpload size={13} strokeWidth={1.75} />
      {:else}
        <Cloud size={13} strokeWidth={1.75} />
      {/if}
    </span>
    <span class="truncate">{label}</span>
  </button>
{/if}

<style>
  .spin {
    display: inline-flex;
    animation: sync-spin 1s linear infinite;
  }
  @keyframes sync-spin {
    to { transform: rotate(360deg); }
  }
</style>
