<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api';
  import { routines as routinesStore } from '$lib/stores/routines.svelte';
  import { notificationSettings } from '$lib/stores/notificationSettings.svelte';
  import { syncLocalReminders, sendTestReminder } from '$lib/localReminders';
  import { isCapacitor } from '$lib/platform';
  import {
    isWebPushSupported, enableWebPush, disableWebPush, isWebPushSubscribed, notificationPermission,
  } from '$lib/webpush';
  import type { NotificationSettings } from '$lib/types';
  import { NOTIFICATION_SOUNDS, playSound, DEFAULT_SOUND_ID } from '$lib/sounds';
  import { Bell, Send, Play } from 'lucide-svelte';

  let settings = $state<NotificationSettings | null>(null);
  let loading = $state(true);
  let saving = $state(false);
  let savedFlash = $state(false);

  // Web Push UI state
  let webPushSupported = $state(false);
  let webPushSubscribed = $state(false);
  let webPushBusy = $state(false);
  let webPushError = $state<string | null>(null);

  // Webhook test state
  let testState = $state<'idle' | 'sending' | 'ok' | 'error'>('idle');
  let testError = $state<string | null>(null);

  // On-device (Android) test-notification state
  const onAndroid = isCapacitor();
  let deviceTestBusy = $state(false);
  let deviceTestMsg = $state<{ ok: boolean; text: string } | null>(null);

  async function sendDeviceTest() {
    deviceTestBusy = true;
    deviceTestMsg = null;
    try {
      const res = await sendTestReminder();
      deviceTestMsg = { ok: res.ok, text: res.message };
    } finally {
      deviceTestBusy = false;
      setTimeout(() => (deviceTestMsg = null), 6000);
    }
  }

  const DAYS = [
    { v: 1, label: 'Mon' }, { v: 2, label: 'Tue' }, { v: 3, label: 'Wed' },
    { v: 4, label: 'Thu' }, { v: 5, label: 'Fri' }, { v: 6, label: 'Sat' }, { v: 7, label: 'Sun' },
  ];

  onMount(async () => {
    webPushSupported = isWebPushSupported();
    // Local-first: show cached settings immediately (works offline, never hangs).
    // init() reconciles with the server in the background for next time.
    await notificationSettings.init();
    settings = structuredClone($state.snapshot(notificationSettings.settings));
    loading = false;
    isWebPushSubscribed().then((v) => (webPushSubscribed = v)).catch(() => {});
  });

  async function save() {
    if (!settings) return;
    saving = true;
    try {
      await notificationSettings.save(settings);
      settings = structuredClone($state.snapshot(notificationSettings.settings));
      savedFlash = true;
      setTimeout(() => (savedFlash = false), 1800);
      // Re-arm the in-app routine scheduler + reschedule on-device alarms.
      void routinesStore.refresh();
      void syncLocalReminders();
    } catch (e) {
      console.warn('save notification settings failed', e);
    } finally {
      saving = false;
    }
  }

  async function toggleWebPush() {
    if (!settings) return;
    webPushBusy = true;
    webPushError = null;
    try {
      if (webPushSubscribed) {
        await disableWebPush();
        webPushSubscribed = false;
        settings.webpush_enabled = false;
      } else {
        const res = await enableWebPush();
        if (!res.ok) {
          webPushError =
            res.error === 'denied' ? 'Permission denied in the browser.'
            : res.error === 'unsupported' ? 'Not supported on this device.'
            : 'Could not enable web push.';
        } else {
          webPushSubscribed = true;
          settings.webpush_enabled = true;
        }
      }
      await save();
    } finally {
      webPushBusy = false;
    }
  }

  async function sendTest() {
    if (!settings) return;
    testState = 'sending';
    testError = null;
    try {
      await api.notifications.testWebhook(settings.webhook);
      testState = 'ok';
      setTimeout(() => (testState = 'idle'), 2500);
    } catch (e) {
      testState = 'error';
      testError = e instanceof Error ? e.message.replace(/^\d+\s/, '') : 'Test failed';
    }
  }

  function toggleWorkday(d: number) {
    if (!settings) return;
    const set = new Set(settings.routines.workdays);
    if (set.has(d)) set.delete(d); else set.add(d);
    settings.routines.workdays = [...set].sort((a, b) => a - b);
  }
</script>

<div class="mx-auto flex h-full max-w-xl flex-col" style="padding-top: env(safe-area-inset-top, 0px);">
  <!-- Header -->
  <div class="flex items-center gap-3 px-5 py-4" style="border-bottom: 1px solid var(--sempa-border);">
    <a href="/settings/accounts"
       class="flex items-center gap-1.5 rounded-lg px-2 py-1.5 text-sm transition-colors"
       style="color: var(--sempa-accent);">
      <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
        <path stroke-linecap="round" d="M19 12H5m7-7-7 7 7 7"/>
      </svg>
      Settings
    </a>
    <h1 class="text-base font-semibold" style="color: var(--sempa-text);">Notifications</h1>
  </div>

  <div class="flex-1 overflow-y-auto px-5 py-6 pb-20">
    {#if loading || !settings}
      <p class="text-sm" style="color: var(--sempa-text-dim);">Loading…</p>
    {:else}

      {#snippet sectionLabel(text: string)}
        <p class="mb-3" style="font-family:monospace; font-size:10.5px; font-weight:700; letter-spacing:0.12em;
           text-transform:uppercase; color:var(--sempa-text-dim)">{text}</p>
      {/snippet}

      {#snippet toggleRow(label: string, desc: string, value: boolean, onChange: (v: boolean) => void, disabled: boolean)}
        <div class="flex items-center gap-3 px-4 py-3.5" style:opacity={disabled ? 0.5 : 1}>
          <div class="min-w-0 flex-1">
            <p class="font-semibold" style="font-size:13.5px; color: var(--sempa-text);">{label}</p>
            <p style="font-size:11.5px; color: var(--sempa-text-soft);">{desc}</p>
          </div>
          <button role="switch" aria-checked={value} aria-label={label} {disabled}
                  onclick={() => onChange(!value)}
                  class="relative h-[26px] w-[44px] shrink-0 rounded-full transition-colors"
                  style="background: {value ? 'var(--sempa-accent)' : 'var(--sempa-border)'};">
            <span class="absolute top-[3px] h-[20px] w-[20px] rounded-full bg-white transition-all"
                  style="left: {value ? '21px' : '3px'};"></span>
          </button>
        </div>
      {/snippet}

      <!-- ── Master ──────────────────────────────────────────────────────── -->
      {@render sectionLabel('General')}
      <section class="mb-7 overflow-hidden rounded-xl border" style="border-color: var(--sempa-border); background: var(--sempa-bg-panel);">
        {@render toggleRow('All notifications', 'Master switch — pause or enable everything.',
          settings.master_enabled, (v) => { settings!.master_enabled = v; void save(); }, false)}
        <div style="border-top: 1px solid var(--sempa-border);"></div>
        {@render toggleRow('Custom alert sound', 'Play a calm tone with reminders.',
          settings.sound_enabled, (v) => { settings!.sound_enabled = v; void save(); }, !settings.master_enabled)}
      </section>

      <!-- ── Sound picker ────────────────────────────────────────────────── -->
      {#if settings.sound_enabled && settings.master_enabled}
        {@const selected = settings.sound_id || DEFAULT_SOUND_ID}
        {@render sectionLabel('Alert sound')}
        <section class="mb-7 overflow-hidden rounded-xl border" style="border-color: var(--sempa-border); background: var(--sempa-bg-panel);">
          {#each NOTIFICATION_SOUNDS as snd, i (snd.id)}
            {@const active = selected === snd.id}
            <div class="flex items-center gap-3 px-4 py-3"
                 style={i < NOTIFICATION_SOUNDS.length - 1 ? 'border-bottom: 1px solid var(--sempa-border);' : ''}>
              <!-- Preview -->
              <button aria-label={'Preview ' + snd.label} onclick={() => playSound(snd.id)}
                      class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg transition-colors"
                      style="background: var(--sempa-bg-main); color: var(--sempa-accent); border: 1px solid var(--sempa-border);">
                <Play size={14} />
              </button>
              <!-- Select (radio row) -->
              <button onclick={() => { settings!.sound_id = snd.id; playSound(snd.id); void save(); }}
                      class="flex min-w-0 flex-1 items-center justify-between text-left">
                <span class="truncate font-semibold" style="font-size:13.5px; color: var(--sempa-text);">{snd.label}</span>
                <span class="ml-3 flex h-[18px] w-[18px] shrink-0 items-center justify-center rounded-full"
                      style="border: 2px solid {active ? 'var(--sempa-accent)' : 'var(--sempa-border)'};">
                  {#if active}
                    <span class="h-[8px] w-[8px] rounded-full" style="background: var(--sempa-accent);"></span>
                  {/if}
                </span>
              </button>
            </div>
          {/each}
        </section>
      {/if}

      <!-- ── Delivery channels ───────────────────────────────────────────── -->
      {@render sectionLabel('Delivery channels')}
      <section class="mb-2 overflow-hidden rounded-xl border" style="border-color: var(--sempa-border); background: var(--sempa-bg-panel);">
        <!-- Web Push -->
        <div class="flex items-center gap-3 px-4 py-3.5">
          <div class="min-w-0 flex-1">
            <p class="font-semibold" style="font-size:13.5px; color: var(--sempa-text);">Web Push</p>
            <p style="font-size:11.5px; color: var(--sempa-text-soft);">
              {#if !webPushSupported}
                Open this app in a browser/PWA to enable.
              {:else if webPushSubscribed}
                Enabled on this device.
              {:else}
                Native notifications on Windows & Android.
              {/if}
            </p>
            {#if webPushError}
              <p style="font-size:11px; color: var(--sempa-danger, #c0392b);">{webPushError}</p>
            {/if}
          </div>
          <button disabled={!webPushSupported || webPushBusy || !settings.master_enabled}
                  onclick={toggleWebPush}
                  class="shrink-0 rounded-lg px-3 py-1.5 font-semibold transition-opacity disabled:opacity-40"
                  style="font-size:12.5px; background: {webPushSubscribed ? 'var(--sempa-bg-main)' : 'var(--sempa-btn-bg)'};
                         color: {webPushSubscribed ? 'var(--sempa-text-soft)' : 'var(--sempa-btn-fg)'};
                         border: 1px solid var(--sempa-border);">
            {webPushBusy ? '…' : webPushSubscribed ? 'Disable' : 'Enable'}
          </button>
        </div>
        <div style="border-top: 1px solid var(--sempa-border);"></div>
        {@render toggleRow('Native (Android FCM)', 'Push to the installed Android app.',
          settings.fcm_enabled, (v) => { settings!.fcm_enabled = v; void save(); }, !settings.master_enabled)}
        <div style="border-top: 1px solid var(--sempa-border);"></div>
        {@render toggleRow('Custom webhook (ntfy / Gotify)', 'POST notifications to a self-hosted service.',
          settings.webhook_enabled, (v) => { settings!.webhook_enabled = v; void save(); }, !settings.master_enabled)}
      </section>

      <!-- ── On-device test (Android) ────────────────────────────────────── -->
      {#if onAndroid}
        <section class="mb-7 rounded-xl border px-4 py-4" style="border-color: var(--sempa-border); background: var(--sempa-bg-panel);">
          <p class="mb-1 font-semibold" style="font-size:13px; color: var(--sempa-text);">Test this device</p>
          <p class="mb-3" style="font-size:11.5px; color: var(--sempa-text-soft);">
            Fires a real notification in a few seconds so you can confirm sound and pop-up work on this phone.
          </p>
          <div class="flex items-center gap-3">
            <button onclick={sendDeviceTest} disabled={deviceTestBusy}
                    class="inline-flex items-center gap-1.5 rounded-lg px-3 py-2 text-sm font-semibold transition-opacity disabled:opacity-40"
                    style="background: var(--sempa-btn-bg); color: var(--sempa-btn-fg);">
              <Bell size={14} />
              {deviceTestBusy ? 'Sending…' : 'Send test notification'}
            </button>
            {#if deviceTestMsg}
              <span class="text-xs" style="color: {deviceTestMsg.ok ? 'var(--sempa-success, #2e7d32)' : 'var(--sempa-danger, #c0392b)'};">
                {deviceTestMsg.text}
              </span>
            {/if}
          </div>
        </section>
      {/if}

      <!-- ── Webhook config ──────────────────────────────────────────────── -->
      {#if settings.webhook_enabled}
        <section class="mb-7 rounded-xl border px-4 py-4" style="border-color: var(--sempa-border); background: var(--sempa-bg-panel);">
          <label class="mb-1 block text-xs font-medium" style="color: var(--sempa-text-soft);" for="wh-endpoint">Endpoint URL</label>
          <input id="wh-endpoint" type="url" placeholder="https://ntfy.sh/ or https://gotify/message?token=…"
                 bind:value={settings.webhook.endpoint}
                 class="mb-3 w-full rounded-lg border px-3 py-2 text-sm"
                 style="border-color: var(--sempa-border); background: var(--sempa-bg-main); color: var(--sempa-text);" />

          <div class="mb-3 grid grid-cols-2 gap-3">
            <div>
              <label class="mb-1 block text-xs font-medium" style="color: var(--sempa-text-soft);" for="wh-topic">Topic (ntfy)</label>
              <input id="wh-topic" type="text" placeholder="my-topic" bind:value={settings.webhook.topic}
                     class="w-full rounded-lg border px-3 py-2 text-sm"
                     style="border-color: var(--sempa-border); background: var(--sempa-bg-main); color: var(--sempa-text);" />
            </div>
            <div>
              <label class="mb-1 block text-xs font-medium" style="color: var(--sempa-text-soft);" for="wh-method">Method</label>
              <input id="wh-method" type="text" placeholder="POST" bind:value={settings.webhook.method}
                     class="w-full rounded-lg border px-3 py-2 text-sm"
                     style="border-color: var(--sempa-border); background: var(--sempa-bg-main); color: var(--sempa-text);" />
            </div>
          </div>

          <div class="mb-3 grid grid-cols-2 gap-3">
            <div>
              <label class="mb-1 block text-xs font-medium" style="color: var(--sempa-text-soft);" for="wh-ah">Auth header</label>
              <input id="wh-ah" type="text" placeholder="Authorization" bind:value={settings.webhook.auth_header}
                     class="w-full rounded-lg border px-3 py-2 text-sm"
                     style="border-color: var(--sempa-border); background: var(--sempa-bg-main); color: var(--sempa-text);" />
            </div>
            <div>
              <label class="mb-1 block text-xs font-medium" style="color: var(--sempa-text-soft);" for="wh-av">Token / value</label>
              <input id="wh-av" type="text" placeholder="Bearer tk_…" bind:value={settings.webhook.auth_value}
                     class="w-full rounded-lg border px-3 py-2 text-sm"
                     style="border-color: var(--sempa-border); background: var(--sempa-bg-main); color: var(--sempa-text);" />
            </div>
          </div>

          <div class="flex items-center gap-3">
            <button onclick={async () => { await save(); await sendTest(); }}
                    disabled={testState === 'sending' || !settings.webhook.endpoint}
                    class="inline-flex items-center gap-1.5 rounded-lg px-3 py-2 text-sm font-semibold transition-opacity disabled:opacity-40"
                    style="background: var(--sempa-btn-bg); color: var(--sempa-btn-fg);">
              <Send size={14} />
              {testState === 'sending' ? 'Sending…' : 'Send test notification'}
            </button>
            {#if testState === 'ok'}
              <span class="text-sm" style="color: var(--sempa-success, #2e7d32);">Sent ✓</span>
            {:else if testState === 'error'}
              <span class="truncate text-xs" style="color: var(--sempa-danger, #c0392b);">{testError}</span>
            {/if}
          </div>
        </section>
      {/if}

      <!-- ── Routines ────────────────────────────────────────────────────── -->
      {@render sectionLabel('In-app routines')}
      <section class="mb-7 rounded-xl border px-4 py-4" style="border-color: var(--sempa-border); background: var(--sempa-bg-panel);">
        <!-- Weekly planning -->
        <div class="mb-4">
          <p class="mb-2 font-semibold" style="font-size:13px; color: var(--sempa-text);">Weekly planning prompt</p>
          <div class="grid grid-cols-2 gap-3">
            <div>
              <label class="mb-1 block text-xs font-medium" style="color: var(--sempa-text-soft);" for="wp-day">Day</label>
              <select id="wp-day" bind:value={settings.routines.weekly_plan_day}
                      class="w-full rounded-lg border px-3 py-2 text-sm"
                      style="border-color: var(--sempa-border); background: var(--sempa-bg-main); color: var(--sempa-text);">
                {#each DAYS as d}
                  <option value={d.v}>{d.label}</option>
                {/each}
              </select>
            </div>
            <div>
              <label class="mb-1 block text-xs font-medium" style="color: var(--sempa-text-soft);" for="wp-time">Time</label>
              <input id="wp-time" type="time" bind:value={settings.routines.weekly_plan_time}
                     class="w-full rounded-lg border px-3 py-2 text-sm"
                     style="border-color: var(--sempa-border); background: var(--sempa-bg-main); color: var(--sempa-text);" />
            </div>
          </div>
        </div>

        <!-- Daily shutdown -->
        <div class="mb-4">
          <p class="mb-2 font-semibold" style="font-size:13px; color: var(--sempa-text);">Daily shutdown review</p>
          <label class="mb-1 block text-xs font-medium" style="color: var(--sempa-text-soft);" for="sd-time">Time</label>
          <input id="sd-time" type="time" bind:value={settings.routines.daily_shutdown_time}
                 class="w-40 rounded-lg border px-3 py-2 text-sm"
                 style="border-color: var(--sempa-border); background: var(--sempa-bg-main); color: var(--sempa-text);" />
        </div>

        <!-- Workdays -->
        <div>
          <p class="mb-2 text-xs font-medium" style="color: var(--sempa-text-soft);">Workdays</p>
          <div class="flex flex-wrap gap-1.5">
            {#each DAYS as d}
              {@const on = settings.routines.workdays.includes(d.v)}
              <button onclick={() => toggleWorkday(d.v)}
                      class="rounded-lg px-2.5 py-1.5 text-xs font-semibold transition-colors"
                      style="background: {on ? 'var(--sempa-accent)' : 'var(--sempa-bg-main)'};
                             color: {on ? 'var(--sempa-btn-fg)' : 'var(--sempa-text-soft)'};
                             border: 1px solid var(--sempa-border);">
                {d.label}
              </button>
            {/each}
          </div>
        </div>
      </section>

      <!-- Save -->
      <div class="flex items-center gap-3">
        <button onclick={save} disabled={saving}
                class="inline-flex items-center gap-1.5 rounded-lg px-4 py-2 text-sm font-semibold transition-opacity disabled:opacity-50"
                style="background: var(--sempa-btn-bg); color: var(--sempa-btn-fg);">
          <Bell size={14} />
          {saving ? 'Saving…' : 'Save settings'}
        </button>
        {#if savedFlash}
          <span class="text-sm" style="color: var(--sempa-success, #2e7d32);">Saved ✓</span>
        {/if}
      </div>

    {/if}
  </div>
</div>
