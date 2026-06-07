<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { goto } from '$app/navigation';
  import { page } from '$app/stores';
  import { api, setServerUrl, getServerUrl, setTauriToken, clearTauriToken, setNativeToken, clearNativeToken, resetApiResolver } from '$lib/api';
  import { isTauri } from '$lib/tauri/bridge';

  let isNative = $state(false);
  let needsServerUrl = $derived(isNative || isTauri());
  let Capacitor: any = null;
  let Browser: any = null;
  let App: any = null;

  let authInfo = $state<{ google_enabled: boolean; password_enabled: boolean } | null>(null);
  let username = $state('');
  let password = $state('');
  let serverUrl = $state('');
  let loading  = $state(false);
  let error    = $state('');
  let showPasswordForm = $state(false);
  let showServerUrl = $state(false);
  let appUrlListener: { remove: () => void } | null = null;

  const redirectTarget = $derived($page.url.searchParams.get('redirect') ?? '/');

  /** Derive the base URL for OAuth redirects (matches api.ts logic). */
  function getBase(): string {
    const envUrl = import.meta.env.VITE_API_URL as string | undefined;
    if (envUrl) return envUrl;
    return getServerUrl();
  }

  async function handleAppUrl(event: { url: string }) {
    try {
      const u = new URL(event.url);
      if (u.hostname !== 'login') return;
      const linkToken = u.searchParams.get('link_token');
      const redirect  = u.searchParams.get('redirect') ?? '/';
      if (!linkToken) return;
      await Browser?.close().catch(() => {});
      const result = await api.auth.nativeFinalize(linkToken);
      // Store session as Bearer token (cookie-based auth is unreliable cross-origin on Android)
      if (result.token) setNativeToken(result.token);
      goto(redirect, { replaceState: true });
    } catch {
      error = 'Google sign-in failed. Please try again.';
    }
  }

  onMount(async () => {
    // Load Capacitor modules dynamically (only on mobile)
    try {
      const capMod = await import('@capacitor/core');
      Capacitor = capMod.Capacitor;
      isNative = Capacitor.isNativePlatform();
      if (isNative) {
        const [browserMod, appMod] = await Promise.all([
          import('@capacitor/browser'),
          import('@capacitor/app'),
        ]);
        Browser = browserMod.Browser;
        App = appMod.App;
      }
    } catch { /* Capacitor not available */ }

    // Tauri Google OAuth callback: backend redirects back to tauri.localhost/login?link_token=X
    // Exchange the one-time token for a Bearer token and proceed into the app.
    if (isTauri()) {
      const linkToken = $page.url.searchParams.get('link_token');
      if (linkToken) {
        loading = true;
        try {
          const result = await api.auth.nativeFinalize(linkToken);
          if (result.token) {
            setTauriToken(result.token);
            resetApiResolver();
          }
          goto(redirectTarget, { replaceState: true });
          return;
        } catch {
          error = 'Google sign-in failed. Please try again.';
          loading = false;
        }
      }
    }

    // Android Capacitor WebView-navigation OAuth callback (fallback when Browser plugin unavailable).
    // Backend redirects to https://localhost/login?link_token=X, which loads this page.
    if (isNative && !isTauri()) {
      const linkToken = $page.url.searchParams.get('link_token');
      if (linkToken) {
        loading = true;
        try {
          const result = await api.auth.nativeFinalize(linkToken);
          if (result.token) setNativeToken(result.token);
          goto(redirectTarget, { replaceState: true });
          return;
        } catch {
          error = 'Google sign-in failed. Please try again.';
          loading = false;
        }
      }
    }

    // On native/Tauri platforms, load any previously saved server URL
    if (needsServerUrl) {
      serverUrl = getServerUrl();
      // If no server URL is configured yet, show the URL field and wait
      if (!serverUrl) {
        showServerUrl = true;
        authInfo = { google_enabled: false, password_enabled: true };
        return;
      }
    }

    await continueAuth();
  });

  async function continueAuth() {
    const errParam = $page.url.searchParams.get('error');
    if (errParam === 'not_allowed') error = 'Your Google account is not authorised for this Sempa instance.';
    else if (errParam) error = 'Google sign-in was cancelled or failed. Please try again.';

    // Check for existing session first
    try {
      const me = await api.auth.me();
      if (me.authenticated) {
        goto(redirectTarget, { replaceState: true });
        return;
      }
    } catch { /* no session — continue */ }

    if (isNative && App) {
      // Cold start: app was opened via the deep link before the listener was registered
      const launch = await App?.getLaunchUrl();
      if (launch?.url?.startsWith('com.clevercode.sempa://login')) {
        await handleAppUrl({ url: launch.url });
        return;
      }
      // Warm start: listen for the deep link callback while app is already running
      appUrlListener = await App?.addListener('appUrlOpen', handleAppUrl);
    }

    // Load auth config
    try {
      authInfo = await api.auth.config();
    } catch {
      authInfo = { google_enabled: false, password_enabled: true };
    }
  }

  onDestroy(() => { appUrlListener?.remove(); });

  function googleSignIn() {
    const base = getBase();
    const params = new URLSearchParams({ redirect: redirectTarget });
    if (isNative && Browser) {
      // Android primary path: Chrome Custom Tab — deep link fires appUrlOpen on return.
      Browser.open({ url: `${base}/api/v1/auth/google?${params}&native=true` });
    } else if (isNative) {
      // Android fallback: Browser plugin unavailable — navigate the WebView directly.
      // Backend will redirect back to this origin's /login?link_token=X.
      const origin = window.location.origin;
      window.location.href = `${base}/api/v1/auth/google?${params}&capacitor_origin=${encodeURIComponent(origin)}`;
    } else if (isTauri()) {
      // Tauri desktop: navigate the WebView; backend redirects back to the Tauri origin.
      const tauriOrigin = window.location.origin;
      window.location.href = `${base}/api/v1/auth/google?${params}&tauri=true&tauri_origin=${encodeURIComponent(tauriOrigin)}`;
    } else {
      window.location.href = `${base}/api/v1/auth/google?${params}`;
    }
  }

  async function submit() {
    if (!username.trim() || !password) return;
    loading = true; error = '';
    try {
      const result = await api.auth.login(username.trim(), password);
      // In Tauri mode, store Bearer token for subsequent requests
      if (isTauri() && result.token) {
        setTauriToken(result.token);
        resetApiResolver(); // force re-evaluation with token
      }
      goto(redirectTarget, { replaceState: true });
    } catch {
      error = 'Invalid username or password.';
    } finally {
      loading = false;
    }
  }

  async function connectServer() {
    if (!serverUrl.trim()) return;
    loading = true; error = '';
    try {
      // Normalize and save URL
      let url = serverUrl.trim();
      if (!url.startsWith('http://') && !url.startsWith('https://')) {
        url = 'https://' + url;
      }
      setServerUrl(url);
      serverUrl = url;
      resetApiResolver(); // switch from localApi to httpApi now that URL is set

      // Test connectivity
      const me = await api.auth.me();
      if (me.authenticated) {
        goto(redirectTarget, { replaceState: true });
        return;
      }
      // Server reachable but not authenticated — proceed to auth
      await continueAuth();
    } catch {
      error = 'Could not connect to server. Check the URL and try again.';
    } finally {
      loading = false;
    }
  }
</script>

<svelte:head><title>Sign in — Sempa</title></svelte:head>

<div class="flex min-h-screen items-center justify-center px-4" style="background: var(--sempa-bg-main);">

  <div class="w-full max-w-sm">

    <!-- Logo -->
    <div class="mb-8 flex flex-col items-center gap-3">
      <div class="flex h-14 w-14 items-center justify-center rounded-2xl shadow-lg"
           style="background: var(--sempa-accent);">
        <svg width="32" height="32" viewBox="0 0 100 100" fill="none" aria-hidden="true">
          <path d="M22,40 a28,28 0 0 0 56,0"
                stroke="white" stroke-width="10" stroke-linecap="round"/>
          <circle cx="50" cy="35" r="8" fill="white"/>
        </svg>
      </div>
      <div class="text-center">
        <h1 class="text-2xl font-semibold tracking-tight" style="color: var(--sempa-text);">sempa</h1>
        <p class="mt-1 text-sm" style="color: var(--sempa-text-soft);">Your personal task manager</p>
      </div>
    </div>

    <div class="rounded-2xl border p-6 shadow-sm space-y-4 animate-scale-in"
         style="border-color: var(--sempa-border); background: var(--sempa-bg-panel);">

      {#if error}
        <div class="rounded-lg px-4 py-3 text-sm text-red-700 bg-red-50 dark:bg-red-950 dark:text-red-300 animate-slide-down">
          {error}
        </div>
      {/if}

      {#if showServerUrl && !getServerUrl()}
        <!-- Server URL configuration (native/mobile) -->
        <form onsubmit={(e) => { e.preventDefault(); connectServer(); }} class="space-y-4">
          <div>
            <label for="server-url" class="mb-1 block text-xs font-medium" style="color: var(--sempa-text-soft);">Server URL</label>
            <input id="server-url" type="url" bind:value={serverUrl}
                   placeholder="https://sempa.example.com"
                   autofocus
                   class="w-full rounded-lg border px-3 py-2 text-sm outline-none"
                   style="border-color: var(--sempa-border); background: var(--sempa-bg-main); color: var(--sempa-text);"
                   onfocus={(e) => { (e.currentTarget as HTMLElement).style.borderColor = 'var(--sempa-accent)'; (e.currentTarget as HTMLElement).style.boxShadow = '0 0 0 2.5px rgba(179,89,46,0.12)'; }}
                   onblur={(e) => { (e.currentTarget as HTMLElement).style.borderColor = 'var(--sempa-border)'; (e.currentTarget as HTMLElement).style.boxShadow = ''; }} />
            <p class="mt-1.5 text-xs" style="color: var(--sempa-text-dim);">
              Enter your self-hosted Sempa server address
            </p>
          </div>
          <button type="submit" disabled={loading || !serverUrl.trim()}
                  class="w-full rounded-lg py-2.5 text-sm font-medium text-white disabled:opacity-40 transition-colors"
                  style="background: var(--sempa-accent);">
            {loading ? 'Connecting…' : 'Connect'}
          </button>
        </form>
      {:else if authInfo === null}
        <!-- Loading -->
        <div class="flex justify-center py-6">
          <div class="h-5 w-5 animate-spin rounded-full border-2 border-gray-200"
               style="border-top-color: var(--sempa-accent);"></div>
        </div>

      {:else if authInfo.google_enabled}
        <!-- Google Sign-In (primary) -->
        <button onclick={googleSignIn}
                class="flex w-full items-center justify-center gap-3 rounded-xl border px-4 py-3
                       text-sm font-medium shadow-sm transition-all hover:shadow-md"
                style="border-color: var(--sempa-border); background: var(--sempa-bg-panel); color: var(--sempa-text);"
                onmousedown={(e) => (e.currentTarget as HTMLElement).style.transform = 'scale(0.97)'}
                onmouseup={(e) => (e.currentTarget as HTMLElement).style.transform = ''}
                onmouseleave={(e) => (e.currentTarget as HTMLElement).style.transform = ''}>
          <!-- Google coloured G -->
          <svg width="20" height="20" viewBox="0 0 24 24" aria-hidden="true">
            <path fill="#4285F4" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"/>
            <path fill="#34A853" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"/>
            <path fill="#FBBC05" d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l3.66-2.84z"/>
            <path fill="#EA4335" d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"/>
          </svg>
          Continue with Google
        </button>

        <!-- Password fallback (collapsed by default) -->
        {#if !showPasswordForm}
          <button onclick={() => showPasswordForm = true}
                  class="w-full text-center text-xs transition-colors"
                  style="color: var(--sempa-text-dim);">
            Sign in with username & password instead
          </button>
        {:else}
          <div class="border-t pt-4 space-y-3" style="border-color: var(--sempa-border);">
            <p class="text-xs font-medium" style="color: var(--sempa-text-soft);">Password sign-in</p>
            <form onsubmit={(e) => { e.preventDefault(); submit(); }} class="space-y-3">
              <div>
                <label for="username" class="mb-1 block text-xs" style="color: var(--sempa-text-soft);">Username</label>
                <input id="username" type="text" bind:value={username} autocomplete="username"
                       class="w-full rounded-lg border px-3 py-2 text-sm outline-none"
                       style="border-color: var(--sempa-border); background: var(--sempa-bg-main); color: var(--sempa-text);"
                       onfocus={(e) => { (e.currentTarget as HTMLElement).style.borderColor = 'var(--sempa-accent)'; (e.currentTarget as HTMLElement).style.boxShadow = '0 0 0 2.5px rgba(179,89,46,0.12)'; }}
                       onblur={(e) => { (e.currentTarget as HTMLElement).style.borderColor = 'var(--sempa-border)'; (e.currentTarget as HTMLElement).style.boxShadow = ''; }} />
              </div>
              <div>
                <label for="password" class="mb-1 block text-xs" style="color: var(--sempa-text-soft);">Password</label>
                <input id="password" type="password" bind:value={password} autocomplete="current-password"
                       class="w-full rounded-lg border px-3 py-2 text-sm outline-none"
                       style="border-color: var(--sempa-border); background: var(--sempa-bg-main); color: var(--sempa-text);"
                       onfocus={(e) => { (e.currentTarget as HTMLElement).style.borderColor = 'var(--sempa-accent)'; (e.currentTarget as HTMLElement).style.boxShadow = '0 0 0 2.5px rgba(179,89,46,0.12)'; }}
                       onblur={(e) => { (e.currentTarget as HTMLElement).style.borderColor = 'var(--sempa-border)'; (e.currentTarget as HTMLElement).style.boxShadow = ''; }} />
              </div>
              <button type="submit" disabled={loading || !username || !password}
                      class="w-full rounded-lg py-2.5 text-sm font-medium text-white disabled:opacity-40 transition-colors"
                      style="background: var(--sempa-accent);">
                {loading ? 'Signing in…' : 'Sign in'}
              </button>
            </form>
          </div>
        {/if}

      {:else}
        <!-- Password-only (no Google configured) -->
        <form onsubmit={(e) => { e.preventDefault(); submit(); }} class="space-y-4">
          <div>
            <label for="username" class="mb-1 block text-xs font-medium" style="color: var(--sempa-text-soft);">Username</label>
            <input id="username" type="text" bind:value={username} autocomplete="username"
                   autofocus
                   class="w-full rounded-lg border px-3 py-2 text-sm outline-none"
                   style="border-color: var(--sempa-border); background: var(--sempa-bg-main); color: var(--sempa-text);"
                   onfocus={(e) => { (e.currentTarget as HTMLElement).style.borderColor = 'var(--sempa-accent)'; (e.currentTarget as HTMLElement).style.boxShadow = '0 0 0 2.5px rgba(179,89,46,0.12)'; }}
                   onblur={(e) => { (e.currentTarget as HTMLElement).style.borderColor = 'var(--sempa-border)'; (e.currentTarget as HTMLElement).style.boxShadow = ''; }} />
          </div>
          <div>
            <label for="password" class="mb-1 block text-xs font-medium" style="color: var(--sempa-text-soft);">Password</label>
            <input id="password" type="password" bind:value={password} autocomplete="current-password"
                   class="w-full rounded-lg border px-3 py-2 text-sm outline-none"
                   style="border-color: var(--sempa-border); background: var(--sempa-bg-main); color: var(--sempa-text);"
                   onfocus={(e) => { (e.currentTarget as HTMLElement).style.borderColor = 'var(--sempa-accent)'; (e.currentTarget as HTMLElement).style.boxShadow = '0 0 0 2.5px rgba(179,89,46,0.12)'; }}
                   onblur={(e) => { (e.currentTarget as HTMLElement).style.borderColor = 'var(--sempa-border)'; (e.currentTarget as HTMLElement).style.boxShadow = ''; }} />
          </div>
          <button type="submit" disabled={loading || !username || !password}
                  class="w-full rounded-lg py-2.5 text-sm font-medium text-white disabled:opacity-40 transition-colors"
                  style="background: var(--sempa-accent);">
            {loading ? 'Signing in…' : 'Sign in'}
          </button>
        </form>
      {/if}
    </div>

    {#if needsServerUrl && getServerUrl()}
      <button onclick={() => { showServerUrl = true; localStorage.removeItem('sempa_server_url'); clearTauriToken(); clearNativeToken(); resetApiResolver(); serverUrl = ''; authInfo = null; error = ''; }}
              class="mt-4 w-full text-center text-xs transition-colors"
              style="color: var(--sempa-text-dim);">
        Change server ({getServerUrl().replace(/^https?:\/\//, '')})
      </button>
    {/if}

    <p class="mt-6 text-center text-xs" style="color: var(--sempa-text-dim);">
      Self-hosted · Your data stays yours
    </p>
  </div>
</div>
