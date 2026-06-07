<script lang="ts">
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import { api } from '$lib/api';
  import type { BackupDestination, BackupRun, BackupSettings, BackupDestinationType } from '$lib/types';

  let loading = $state(true);
  let saving = $state(false);
  let error = $state('');
  let notice = $state('');

  // Form state
  let enabled = $state(false);
  let scheduleHour = $state(3);
  let retention = $state(7);
  let securityMode = $state<'none' | 'encrypt' | 'exclude_secrets'>('none');
  let passphrase = $state('');           // only sent when non-empty
  let hasPassphrase = $state(false);
  let destinations = $state<BackupDestination[]>([]);

  let runs = $state<BackupRun[]>([]);
  let lastRunAt = $state<string | null>(null);
  let lastStatus = $state<string | null>(null);
  let driveConnected = $state(false);
  let googleOAuth = $state(false);

  // Manual restore
  let restoreFile = $state<File | null>(null);
  let restorePassphrase = $state('');
  let restoreConfirm = $state(false);
  let restoring = $state(false);
  let restorePct = $state<number | null>(null);

  let runningNow = $state(false);
  let testResults = $state<Record<string, string>>({});

  onMount(() => {
    void load();
    if ($page.url.searchParams.get('drive') === 'connected') {
      notice = 'Google Drive connected.';
    }
  });

  async function load() {
    loading = true; error = '';
    try {
      const r = await api.backup.getSettings();
      const s = r.settings;
      enabled = s.enabled;
      scheduleHour = s.schedule_hour;
      retention = s.retention;
      securityMode = s.security_mode;
      hasPassphrase = s.has_passphrase;
      destinations = parseDestinations(s.destinations);
      runs = r.runs ?? [];
      lastRunAt = s.last_run_at;
      lastStatus = s.last_status;
      driveConnected = r.drive_connected;
      googleOAuth = r.google_oauth;
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to load backup settings';
    } finally {
      loading = false;
    }
  }

  function parseDestinations(raw: string): BackupDestination[] {
    try { const v = JSON.parse(raw || '[]'); return Array.isArray(v) ? v : []; }
    catch { return []; }
  }

  const HOURS = Array.from({ length: 24 }, (_, h) => ({
    value: h,
    label: new Date(2000, 0, 1, h).toLocaleTimeString(undefined, { hour: 'numeric' }),
  }));

  const destTypeLabel: Record<BackupDestinationType, string> = {
    drive: 'Google Drive', s3: 'S3-compatible', webdav: 'WebDAV / Nextcloud', local: 'Local folder',
  };

  function addDestination(type: BackupDestinationType) {
    destinations = [...destinations, {
      id: crypto.randomUUID(), type, name: destTypeLabel[type], enabled: true,
      region: type === 's3' ? 'us-east-1' : undefined,
    }];
  }
  function removeDestination(id: string) {
    destinations = destinations.filter(d => d.id !== id);
  }

  async function save() {
    saving = true; error = ''; notice = '';
    try {
      const payload: Parameters<typeof api.backup.updateSettings>[0] = {
        enabled,
        schedule_hour: scheduleHour,
        retention: Math.max(1, retention || 1),
        security_mode: securityMode,
        destinations,
      };
      if (passphrase.trim()) payload.passphrase = passphrase;
      const r = await api.backup.updateSettings(payload);
      const s = r.settings;
      hasPassphrase = s.has_passphrase;
      destinations = parseDestinations(s.destinations);
      passphrase = '';
      notice = 'Settings saved.';
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to save';
    } finally {
      saving = false;
    }
  }

  async function testDestination(id: string) {
    testResults = { ...testResults, [id]: '…' };
    try {
      const r = await api.backup.test(id);
      testResults = { ...testResults, [id]: r.ok ? `OK · ${r.existing_backups ?? 0} backups found` : `Failed: ${r.error}` };
    } catch (e) {
      testResults = { ...testResults, [id]: `Failed: ${e instanceof Error ? e.message : 'error'}` };
    }
  }

  async function runNow() {
    runningNow = true; error = ''; notice = '';
    try {
      const r = await api.backup.run();
      if (r.error) error = `Backup ran with errors: ${r.error}`;
      else notice = 'Backup completed.';
      await load();
    } catch (e) {
      error = e instanceof Error ? e.message : 'Backup failed';
    } finally {
      runningNow = false;
    }
  }

  function downloadBackup() {
    window.open(api.backup.downloadUrl(), '_blank');
  }

  function connectDrive() {
    window.location.href = api.backup.driveAuthUrl();
  }
  async function disconnectDrive() {
    await api.backup.driveDisconnect();
    driveConnected = false;
  }

  function onRestorePick(e: Event) {
    const input = e.target as HTMLInputElement;
    restoreFile = input.files?.[0] ?? null;
    restoreConfirm = false;
  }

  async function doRestore() {
    if (!restoreFile) return;
    restoring = true; error = ''; notice = ''; restorePct = 0;
    try {
      await api.backup.restore(restoreFile, restorePassphrase || undefined, (p) => (restorePct = p));
      notice = 'Restore complete. Reloading…';
      setTimeout(() => location.reload(), 1200);
    } catch (e) {
      error = e instanceof Error ? e.message : 'Restore failed';
    } finally {
      restoring = false;
      restorePct = null;
      restoreConfirm = false;
    }
  }

  function fmtBytes(n: number | null): string {
    if (!n) return '—';
    if (n < 1024 * 1024) return `${(n / 1024).toFixed(0)} KB`;
    if (n < 1024 * 1024 * 1024) return `${(n / (1024 * 1024)).toFixed(1)} MB`;
    return `${(n / (1024 * 1024 * 1024)).toFixed(2)} GB`;
  }
  function fmtDate(s: string | null): string {
    if (!s) return 'never';
    const d = new Date(s);
    return d.toLocaleString(undefined, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' });
  }
</script>

<div class="mx-auto max-w-2xl px-4 py-6 pb-24">
  <a href="/settings/accounts" class="mb-6 inline-flex items-center gap-1.5 text-sm transition-colors"
     style="color: var(--sempa-text-dim);">
    <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
      <path stroke-linecap="round" d="m15 18-6-6 6-6"/>
    </svg>
    Settings
  </a>

  <h1 class="mb-1 text-xl font-bold" style="color: var(--sempa-text);">Backup &amp; Restore</h1>
  <p class="mb-6 text-sm" style="color: var(--sempa-text-soft);">
    Bundle everything — tasks, objectives, plans, and attached files — into one file you can restore anywhere.
  </p>

  {#if notice}
    <p class="mb-4 rounded-lg px-3 py-2 text-sm" style="background: color-mix(in srgb, #22c55e 12%, transparent); color: #16a34a;">{notice}</p>
  {/if}
  {#if error}
    <p class="mb-4 rounded-lg bg-red-50 px-3 py-2 text-sm text-red-600 dark:bg-red-950 dark:text-red-400">{error}</p>
  {/if}

  {#if loading}
    <p class="text-sm" style="color: var(--sempa-text-dim);">Loading…</p>
  {:else}
    <!-- Status -->
    <section class="mb-6 rounded-xl border px-5 py-4" style="border-color: var(--sempa-border); background: var(--sempa-bg-panel);">
      <div class="flex items-center justify-between gap-3">
        <div>
          <p class="text-sm font-semibold" style="color: var(--sempa-text);">Last backup</p>
          <p class="text-xs" style="color: var(--sempa-text-soft);">
            {#if lastStatus === 'error'}
              <span class="text-red-500">Failed</span> · {fmtDate(lastRunAt)}
            {:else if lastRunAt}
              <span class="text-green-600">Success</span> · {fmtDate(lastRunAt)}
            {:else}
              No backups yet
            {/if}
          </p>
        </div>
        <div class="flex gap-2">
          <button onclick={downloadBackup}
                  class="rounded-lg border px-3 py-1.5 text-xs font-medium transition-colors"
                  style="border-color: var(--sempa-border); color: var(--sempa-text);">
            Download now
          </button>
          <button onclick={runNow} disabled={runningNow}
                  class="rounded-lg px-3 py-1.5 text-xs font-medium text-white transition-colors disabled:opacity-50"
                  style="background: var(--sempa-accent);">
            {runningNow ? 'Running…' : 'Back up now'}
          </button>
        </div>
      </div>
    </section>

    <!-- Schedule -->
    <section class="mb-6 rounded-xl border px-5 py-4" style="border-color: var(--sempa-border); background: var(--sempa-bg-panel);">
      <label class="flex items-center justify-between">
        <span class="text-sm font-semibold" style="color: var(--sempa-text);">Automatic daily backup</span>
        <input type="checkbox" bind:checked={enabled} class="h-4 w-4 accent-[var(--sempa-accent)]" />
      </label>
      <div class="mt-4 grid grid-cols-2 gap-4">
        <div>
          <label class="mb-1 block text-xs font-medium" style="color: var(--sempa-text-soft);" for="hour">Run at</label>
          <select id="hour" bind:value={scheduleHour}
                  class="w-full rounded-lg border px-2 py-1.5 text-sm" style="border-color: var(--sempa-border); background: var(--sempa-bg);">
            {#each HOURS as h}<option value={h.value}>{h.label}</option>{/each}
          </select>
        </div>
        <div>
          <label class="mb-1 block text-xs font-medium" style="color: var(--sempa-text-soft);" for="ret">Keep last</label>
          <div class="flex items-center gap-2">
            <input id="ret" type="number" min="1" bind:value={retention}
                   class="w-20 rounded-lg border px-2 py-1.5 text-sm" style="border-color: var(--sempa-border); background: var(--sempa-bg);" />
            <span class="text-xs" style="color: var(--sempa-text-soft);">backups</span>
          </div>
        </div>
      </div>
    </section>

    <!-- Security -->
    <section class="mb-6 rounded-xl border px-5 py-4" style="border-color: var(--sempa-border); background: var(--sempa-bg-panel);">
      <p class="mb-3 text-sm font-semibold" style="color: var(--sempa-text);">Security</p>
      <div class="space-y-2">
        {#each [
          { v: 'none', t: 'Standard', d: 'Plain bundle. Includes your integration tokens.' },
          { v: 'encrypt', t: 'Encrypt with passphrase', d: 'AES-256. You need the passphrase to restore — store it safely.' },
          { v: 'exclude_secrets', t: 'Exclude integration secrets', d: 'Everything except Gmail/Jira/Fastmail tokens. Re-link them after a restore.' },
        ] as opt}
          <label class="flex cursor-pointer items-start gap-2.5 rounded-lg border px-3 py-2.5"
                 style="border-color: {securityMode === opt.v ? 'var(--sempa-accent)' : 'var(--sempa-border)'};">
            <input type="radio" name="security" value={opt.v} bind:group={securityMode} class="mt-0.5 accent-[var(--sempa-accent)]" />
            <span>
              <span class="block text-sm font-medium" style="color: var(--sempa-text);">{opt.t}</span>
              <span class="block text-xs" style="color: var(--sempa-text-soft);">{opt.d}</span>
            </span>
          </label>
        {/each}
      </div>
      {#if securityMode === 'encrypt'}
        <div class="mt-3">
          <label class="mb-1 block text-xs font-medium" style="color: var(--sempa-text-soft);" for="pass">
            Passphrase {#if hasPassphrase}<span style="color: var(--sempa-text-dim);">— leave blank to keep current</span>{/if}
          </label>
          <input id="pass" type="password" bind:value={passphrase} autocomplete="new-password"
                 placeholder={hasPassphrase ? '••••••••' : 'Enter a passphrase'}
                 class="w-full rounded-lg border px-3 py-2 text-sm" style="border-color: var(--sempa-border); background: var(--sempa-bg);" />
        </div>
      {/if}
    </section>

    <!-- Destinations -->
    <section class="mb-6 rounded-xl border px-5 py-4" style="border-color: var(--sempa-border); background: var(--sempa-bg-panel);">
      <p class="mb-1 text-sm font-semibold" style="color: var(--sempa-text);">Destinations</p>
      <p class="mb-3 text-xs" style="color: var(--sempa-text-soft);">Where automatic backups are sent. Manual downloads always work without one.</p>

      <div class="space-y-3">
        {#each destinations as dest (dest.id)}
          <div class="rounded-lg border p-3" style="border-color: var(--sempa-border);">
            <div class="mb-2 flex items-center justify-between gap-2">
              <span class="rounded-md px-1.5 py-0.5 text-[10px] font-semibold uppercase tracking-wide"
                    style="background: var(--sempa-accent-bg); color: var(--sempa-accent);">{destTypeLabel[dest.type]}</span>
              <div class="flex items-center gap-2">
                <label class="flex items-center gap-1.5 text-xs" style="color: var(--sempa-text-soft);">
                  <input type="checkbox" bind:checked={dest.enabled} class="h-3.5 w-3.5 accent-[var(--sempa-accent)]" /> Enabled
                </label>
                <button onclick={() => removeDestination(dest.id)} class="text-xs text-red-400 hover:text-red-500">Remove</button>
              </div>
            </div>

            <input bind:value={dest.name} placeholder="Name"
                   class="mb-2 w-full rounded-md border px-2 py-1.5 text-sm" style="border-color: var(--sempa-border); background: var(--sempa-bg);" />

            {#if dest.type === 'local'}
              <input bind:value={dest.path} placeholder="/path/on/server (e.g. /data/backups)"
                     class="w-full rounded-md border px-2 py-1.5 text-sm" style="border-color: var(--sempa-border); background: var(--sempa-bg);" />
            {:else if dest.type === 'webdav'}
              <div class="grid gap-2">
                <input bind:value={dest.url} placeholder="https://nextcloud.example.com/remote.php/dav/files/me/Backups"
                       class="rounded-md border px-2 py-1.5 text-sm" style="border-color: var(--sempa-border); background: var(--sempa-bg);" />
                <div class="grid grid-cols-2 gap-2">
                  <input bind:value={dest.username} placeholder="Username"
                         class="rounded-md border px-2 py-1.5 text-sm" style="border-color: var(--sempa-border); background: var(--sempa-bg);" />
                  <input bind:value={dest.password} type="password" placeholder="Password / app token"
                         class="rounded-md border px-2 py-1.5 text-sm" style="border-color: var(--sempa-border); background: var(--sempa-bg);" />
                </div>
              </div>
            {:else if dest.type === 's3'}
              <div class="grid gap-2">
                <div class="grid grid-cols-2 gap-2">
                  <input bind:value={dest.bucket} placeholder="Bucket"
                         class="rounded-md border px-2 py-1.5 text-sm" style="border-color: var(--sempa-border); background: var(--sempa-bg);" />
                  <input bind:value={dest.region} placeholder="Region (us-east-1)"
                         class="rounded-md border px-2 py-1.5 text-sm" style="border-color: var(--sempa-border); background: var(--sempa-bg);" />
                </div>
                <input bind:value={dest.endpoint} placeholder="Endpoint (blank for AWS; e.g. https://s3.us-west-002.backblazeb2.com)"
                       class="rounded-md border px-2 py-1.5 text-sm" style="border-color: var(--sempa-border); background: var(--sempa-bg);" />
                <input bind:value={dest.prefix} placeholder="Key prefix (optional, e.g. sempa/)"
                       class="rounded-md border px-2 py-1.5 text-sm" style="border-color: var(--sempa-border); background: var(--sempa-bg);" />
                <div class="grid grid-cols-2 gap-2">
                  <input bind:value={dest.access_key_id} placeholder="Access key ID"
                         class="rounded-md border px-2 py-1.5 text-sm" style="border-color: var(--sempa-border); background: var(--sempa-bg);" />
                  <input bind:value={dest.secret_access_key} type="password" placeholder="Secret access key"
                         class="rounded-md border px-2 py-1.5 text-sm" style="border-color: var(--sempa-border); background: var(--sempa-bg);" />
                </div>
              </div>
            {:else if dest.type === 'drive'}
              <div class="space-y-2">
                {#if driveConnected}
                  <p class="text-xs" style="color: var(--sempa-text-soft);">
                    <span class="text-green-600">Connected.</span> Backups go to a “Sempa Backups” folder.
                    <button onclick={disconnectDrive} class="ml-1 text-red-400 hover:text-red-500">Disconnect</button>
                  </p>
                {:else if googleOAuth}
                  <button onclick={connectDrive}
                          class="rounded-md border px-3 py-1.5 text-xs font-medium" style="border-color: var(--sempa-border); color: var(--sempa-text);">
                    Connect Google Drive
                  </button>
                {:else}
                  <p class="text-xs text-amber-600">Google OAuth isn’t configured on this server (set GMAIL_CLIENT_ID/SECRET).</p>
                {/if}
                <input bind:value={dest.folder_id} placeholder="Drive folder ID (optional — blank = Drive root)"
                       class="w-full rounded-md border px-2 py-1.5 text-sm" style="border-color: var(--sempa-border); background: var(--sempa-bg);" />
              </div>
            {/if}

            <div class="mt-2 flex items-center gap-3">
              <button onclick={() => testDestination(dest.id)} class="text-xs font-medium text-blue-500 hover:text-blue-600">Test connection</button>
              {#if testResults[dest.id]}<span class="text-xs" style="color: var(--sempa-text-soft);">{testResults[dest.id]}</span>{/if}
            </div>
          </div>
        {/each}
      </div>

      <div class="mt-3 flex flex-wrap gap-2">
        {#each (['local','drive','s3','webdav'] as BackupDestinationType[]) as t}
          <button onclick={() => addDestination(t)}
                  class="rounded-lg border px-3 py-1.5 text-xs font-medium transition-colors"
                  style="border-color: var(--sempa-border); color: var(--sempa-text-soft);">
            + {destTypeLabel[t]}
          </button>
        {/each}
      </div>
    </section>

    <div class="mb-8 flex justify-end">
      <button onclick={save} disabled={saving}
              class="rounded-lg px-5 py-2 text-sm font-medium text-white transition-colors disabled:opacity-50"
              style="background: var(--sempa-accent);">
        {saving ? 'Saving…' : 'Save settings'}
      </button>
    </div>

    <!-- Restore -->
    <section class="mb-6 rounded-xl border px-5 py-4" style="border-color: #f59e0b55; background: color-mix(in srgb, #f59e0b 6%, var(--sempa-bg-panel));">
      <p class="mb-1 text-sm font-semibold" style="color: var(--sempa-text);">Restore from a backup</p>
      <p class="mb-3 text-xs" style="color: var(--sempa-text-soft);">
        This <strong>erases all current data</strong> and replaces it with the backup. Encrypted backups need their passphrase.
      </p>
      <input type="file" accept=".zip,.enc" onchange={onRestorePick}
             class="mb-2 block w-full text-xs" />
      {#if restoreFile}
        <input type="password" bind:value={restorePassphrase} placeholder="Passphrase (only if encrypted)"
               class="mb-2 w-full rounded-md border px-2 py-1.5 text-sm" style="border-color: var(--sempa-border); background: var(--sempa-bg);" />
        {#if restorePct !== null}
          <div class="mb-2 h-1.5 w-full overflow-hidden rounded-full bg-gray-200 dark:bg-gray-700">
            <div class="h-full rounded-full bg-amber-500 transition-all" style="width: {restorePct}%"></div>
          </div>
        {/if}
        {#if !restoreConfirm}
          <button onclick={() => restoreConfirm = true} disabled={restoring}
                  class="rounded-lg px-4 py-2 text-sm font-medium text-white" style="background: #f59e0b;">
            Restore “{restoreFile.name}”
          </button>
        {:else}
          <div class="flex items-center gap-2">
            <span class="text-sm" style="color: var(--sempa-text);">Erase everything and restore?</span>
            <button onclick={doRestore} disabled={restoring}
                    class="rounded-lg px-3 py-1.5 text-sm font-medium text-white disabled:opacity-50" style="background: #dc2626;">
              {restoring ? 'Restoring…' : 'Yes, restore'}
            </button>
            <button onclick={() => restoreConfirm = false} class="text-sm" style="color: var(--sempa-text-dim);">Cancel</button>
          </div>
        {/if}
      {/if}
    </section>

    <!-- History -->
    {#if runs.length}
      <section class="rounded-xl border px-5 py-4" style="border-color: var(--sempa-border); background: var(--sempa-bg-panel);">
        <p class="mb-3 text-sm font-semibold" style="color: var(--sempa-text);">Recent backups</p>
        <div class="space-y-1.5">
          {#each runs as run}
            <div class="flex items-center justify-between gap-2 text-xs">
              <span class="flex items-center gap-2">
                <span class="h-1.5 w-1.5 rounded-full {run.status === 'success' ? 'bg-green-500' : 'bg-red-500'}"></span>
                <span style="color: var(--sempa-text);">{fmtDate(run.started_at)}</span>
                <span style="color: var(--sempa-text-dim);">· {run.trigger}</span>
              </span>
              <span style="color: var(--sempa-text-soft);">
                {run.status === 'error' ? (run.error ?? 'error') : fmtBytes(run.size_bytes)}
              </span>
            </div>
          {/each}
        </div>
      </section>
    {/if}
  {/if}
</div>
