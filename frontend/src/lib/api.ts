import { isTauri } from './tauri/bridge';

import type {
  CreateObjectiveInput,
  CreateTaskInput,
  DailyPlan,
  FastmailEmail,
  GmailIntegrationConfig,
  JiraIntegrationConfig,
  Objective,
  PomodoroSession,
  SyncResult,
  TagDefinition,
  Task,
  UpdateObjectiveInput,
  UpdateTaskInput,
  UpsertPlanInput,
  WeekReview,
} from './types';

// Resolve the API base URL:
// 1. Build-time env var (dev): VITE_API_URL
// 2. Runtime user-configured server (mobile/native): stored in localStorage
// 3. Fallback: empty string → relative URLs (web served by Go)
function getBaseUrl(): string {
  const envUrl = import.meta.env.VITE_API_URL as string | undefined;
  if (envUrl) return envUrl;
  if (typeof localStorage !== 'undefined') {
    const stored = localStorage.getItem('sempa_server_url');
    if (stored) return stored;
  }
  return '';
}

/** Update the stored server URL (call from login/settings). */
export function setServerUrl(url: string) {
  const trimmed = url.replace(/\/+$/, '');
  localStorage.setItem('sempa_server_url', trimmed);
}

/** Read the currently configured server URL. */
export function getServerUrl(): string {
  return typeof localStorage !== 'undefined'
    ? localStorage.getItem('sempa_server_url') ?? ''
    : '';
}

const TAURI_TOKEN_KEY = 'sempa_tauri_token';
export function getTauriToken(): string {
  return typeof localStorage !== 'undefined' ? localStorage.getItem(TAURI_TOKEN_KEY) ?? '' : '';
}
export function setTauriToken(token: string) {
  localStorage.setItem(TAURI_TOKEN_KEY, token);
}
export function clearTauriToken() {
  localStorage.removeItem(TAURI_TOKEN_KEY);
}

// Native mobile token (Android Capacitor) — same Bearer auth pattern as Tauri
const NATIVE_TOKEN_KEY = 'sempa_native_token';
export function getNativeToken(): string {
  return typeof localStorage !== 'undefined' ? localStorage.getItem(NATIVE_TOKEN_KEY) ?? '' : '';
}
export function setNativeToken(token: string) {
  localStorage.setItem(NATIVE_TOKEN_KEY, token);
}
export function clearNativeToken() {
  localStorage.removeItem(NATIVE_TOKEN_KEY);
}

async function req<T>(path: string, init?: RequestInit): Promise<T> {
  const base = getBaseUrl();
  const extraHeaders: Record<string, string> = { 'Content-Type': 'application/json' };

  // Use Bearer token for Tauri desktop and Android native (avoids cross-origin cookie issues)
  const bearerToken = isTauri() ? getTauriToken() : getNativeToken();
  if (bearerToken) extraHeaders['Authorization'] = `Bearer ${bearerToken}`;

  const res = await fetch(`${base}${path}`, {
    ...init,
    headers: { ...extraHeaders, ...(init?.headers as Record<string, string> ?? {}) },
    // Omit credentials when using Bearer auth; web browser sessions still use cookies
    credentials: bearerToken ? 'omit' : 'include',
  });
  if (!res.ok) {
    const body = await res.text();
    throw new Error(`${res.status} ${res.statusText}: ${body}`);
  }
  if (res.status === 204) return undefined as T;
  return res.json();
}

const body = (data: unknown) => JSON.stringify(data);

const httpApi = {
  setup: {
    status: () => req<{ done: boolean }>('/api/v1/setup/status'),
    complete: () => req<{ done: boolean }>('/api/v1/setup/complete', { method: 'POST' }),
  },

  auth: {
    config: () => req<{ google_enabled: boolean; password_enabled: boolean }>('/api/v1/auth/config'),
    me: () => req<{ authenticated: boolean; auth_enabled: boolean; google_enabled: boolean; email?: string; username?: string }>('/api/v1/auth/me'),
    login: (username: string, password: string) =>
      req<{ status: string; token?: string }>('/api/v1/auth/login', { method: 'POST', body: body({ username, password }) }),
    logout: () => req<void>('/api/v1/auth/logout', { method: 'POST' }),
    nativeFinalize: (linkToken: string) =>
      req<{ status: string; token?: string }>('/api/v1/auth/native/finalize', { method: 'POST', body: body({ link_token: linkToken }) }),
  },

  tasks: {
    listByDate:   (date: string)        => req<Task[]>(`/api/v1/tasks?date=${date}`),
    listByWeek:   (weekStart: string)   => req<Task[]>(`/api/v1/tasks?week_start=${weekStart}`),
    listBacklog:  ()                    => req<Task[]>('/api/v1/tasks'),
    listByRecurrenceOrigin: (originId: string) => req<Task[]>(`/api/v1/tasks?recurrence_origin=${originId}`),
    listBySource: (source: string)      => req<Task[]>(`/api/v1/tasks?source=${source}`),
    listByParent: (parentId: string)    => req<Task[]>(`/api/v1/tasks?parent_id=${parentId}`),
    get:          (id: string)        => req<Task>(`/api/v1/tasks/${id}`),
    create: (input: CreateTaskInput) =>
      req<Task>('/api/v1/tasks', { method: 'POST', body: body(input) }),
    update: (id: string, patch: UpdateTaskInput) =>
      req<Task>(`/api/v1/tasks/${id}`, { method: 'PATCH', body: body(patch) }),
    delete: (id: string) => req<void>(`/api/v1/tasks/${id}`, { method: 'DELETE' }),
  },

  devices: {
    register: (token: string, platform: string) =>
      req<any>('/api/v1/devices', { method: 'POST', body: body({ token, platform }) }),
    unregister: (token: string) =>
      req<void>('/api/v1/devices', { method: 'DELETE', body: body({ token }) }),
  },

  objectives: {
    listByWeek: (weekStart: string) =>
      req<Objective[]>(`/api/v1/objectives?week_start=${weekStart}`),
    get: (id: string) => req<Objective>(`/api/v1/objectives/${id}`),
    create: (input: CreateObjectiveInput) =>
      req<Objective>('/api/v1/objectives', { method: 'POST', body: body(input) }),
    update: (id: string, patch: UpdateObjectiveInput) =>
      req<Objective>(`/api/v1/objectives/${id}`, { method: 'PATCH', body: body(patch) }),
    delete: (id: string) => req<void>(`/api/v1/objectives/${id}`, { method: 'DELETE' }),
  },

  plans: {
    get: (date: string) => req<DailyPlan>(`/api/v1/plans/${date}`),
    upsert: (date: string, input: UpsertPlanInput) =>
      req<DailyPlan>(`/api/v1/plans/${date}`, { method: 'PUT', body: body(input) }),
  },

  pomodoros: {
    create: (input: {
      task_id: string;
      duration_minutes: number;
      started_at: string;
      completed_at?: string;
      was_completed: boolean;
    }) => req<PomodoroSession>('/api/v1/pomodoros', { method: 'POST', body: body(input) }),
    listByTask: (taskId: string) => req<PomodoroSession[]>(`/api/v1/pomodoros?task_id=${taskId}`),
  },

  tags: {
    list: () => req<TagDefinition[]>('/api/v1/tags'),
    create: (name: string, color?: string) =>
      req<TagDefinition>('/api/v1/tags', { method: 'POST', body: body({ name, color }) }),
    update: (id: string, color: string) =>
      req<TagDefinition>(`/api/v1/tags/${id}`, { method: 'PATCH', body: body({ color }) }),
    delete: (id: string) => req<void>(`/api/v1/tags/${id}`, { method: 'DELETE' }),
  },

  recurring: {
    list: () => req<Task[]>('/api/v1/tasks/recurring'),
    delete: (id: string) => req<void>(`/api/v1/tasks/${id}`, { method: 'DELETE' }),
  },

  weeks: {
    getReview:    (weekStart: string) => req<WeekReview>(`/api/v1/weeks/${weekStart}/review`),
    upsertReview: (weekStart: string, data: { wins: string | null; challenges: string | null; next_focus: string | null }) =>
      req<WeekReview>(`/api/v1/weeks/${weekStart}/review`, { method: 'PUT', body: body(data) }),
  },

  ical: {
    listSubscriptions: () => req<import('./types').ICalSubscription[]>('/api/v1/ical/subscriptions'),
    createSubscription: (data: { name: string; url: string; color?: string }) =>
      req<import('./types').ICalSubscription>('/api/v1/ical/subscriptions', { method: 'POST', body: body(data) }),
    deleteSubscription: (id: string) =>
      req<void>(`/api/v1/ical/subscriptions/${id}`, { method: 'DELETE' }),
    syncSubscription: (id: string) =>
      req<{ status: string }>(`/api/v1/ical/subscriptions/${id}/sync`, { method: 'POST' }),
    listEvents: (date: string) =>
      req<import('./types').ICalEvent[]>(`/api/v1/ical/events?date=${date}`),
  },

  integrations: {
    jira: {
      get: () => req<JiraIntegrationConfig>('/api/v1/integrations/jira'),
      save: (cfg: { host: string; email: string; api_token: string; jql?: string }) =>
        req<JiraIntegrationConfig>('/api/v1/integrations/jira', { method: 'PUT', body: body(cfg) }),
      test: () => req<{ status: string }>('/api/v1/integrations/jira/test', { method: 'POST' }),
      sync: () => req<SyncResult>('/api/v1/integrations/jira/sync', { method: 'POST' }),
      delete: () => req<void>('/api/v1/integrations/jira', { method: 'DELETE' }),
      getStatuses: () =>
        req<{ id: string; name: string; statusCategory: { key: string } }[]>(
          '/api/v1/integrations/jira/statuses'),
      getIssue: (key: string) =>
        req<any>(`/api/v1/integrations/jira/issues/${key}`),
      getTransitions: (key: string) =>
        req<{ id: string; name: string; to: { statusCategory: { key: string } } }[]>(
          `/api/v1/integrations/jira/issues/${key}/transitions`),
      transition: (key: string, transitionId: string) =>
        req<void>(`/api/v1/integrations/jira/issues/${key}/transition`,
          { method: 'POST', body: body({ transition_id: transitionId }) }),
    },
    gmail: {
      get: () => req<GmailIntegrationConfig>('/api/v1/integrations/gmail'),
      authUrl: (withCalendar = false) =>
        `${getBaseUrl()}/api/v1/integrations/gmail/auth${withCalendar ? '?calendar=1' : ''}`,
      updateLabels: (labels: string[]) =>
        req<unknown>('/api/v1/integrations/gmail/labels', { method: 'PATCH', body: body({ labels }) }),
      sync: () => req<SyncResult>('/api/v1/integrations/gmail/sync', { method: 'POST' }),
      delete: () => req<void>('/api/v1/integrations/gmail', { method: 'DELETE' }),
    },

    calendar: {
      get: () => req<{ connected: boolean; email?: string; calendar_ids?: string[]; last_synced_at?: string }>('/api/v1/integrations/calendar'),
      toggle: (enabled: boolean, calendarIds?: string[]) =>
        req<{ enabled: boolean }>('/api/v1/integrations/calendar', {
          method: 'PATCH', body: body({ enabled, calendar_ids: calendarIds }),
        }),
      sync: (date?: string) =>
        req<SyncResult>(`/api/v1/integrations/calendar/sync${date ? `?date=${date}` : ''}`, { method: 'POST' }),
    },

    fastmail: {
      get: () => req<{ connected: boolean; email?: string; last_synced_at?: string }>('/api/v1/integrations/fastmail'),
      save: (email: string, app_password: string) =>
        req<unknown>('/api/v1/integrations/fastmail', { method: 'PUT', body: body({ email, app_password }) }),
      sync: () => req<SyncResult>('/api/v1/integrations/fastmail/sync', { method: 'POST' }),
      delete: () => req<void>('/api/v1/integrations/fastmail', { method: 'DELETE' }),
      emails: () => req<FastmailEmail[]>('/api/v1/integrations/fastmail/emails'),
      archivedEmails: () => req<FastmailEmail[]>('/api/v1/integrations/fastmail/emails/archived'),
      toTask: (id: string, subject: string) =>
        req<Task>(`/api/v1/integrations/fastmail/emails/${id}/to-task`, { method: 'POST', body: body({ subject }) }),
      archive: (id: string) =>
        req<void>(`/api/v1/integrations/fastmail/emails/${id}/archive`, { method: 'POST' }),
      unarchive: (id: string) =>
        req<void>(`/api/v1/integrations/fastmail/emails/${id}/unarchive`, { method: 'POST' }),
      calendar: {
        get: () => req<{ connected: boolean; enabled: boolean; last_synced_at?: string | null }>('/api/v1/integrations/fastmail/calendar'),
        toggle: (enabled: boolean) =>
          req<{ enabled: boolean }>('/api/v1/integrations/fastmail/calendar', { method: 'PATCH', body: body({ enabled }) }),
        sync: () => req<{ synced: number; from: string; to: string }>('/api/v1/integrations/fastmail/calendar/sync', { method: 'POST' }),
      },
    },

    taskInbox: {
      get: () => req<{
        connected: boolean; email?: string; inbox_address?: string;
        allowed_senders?: string[]; last_synced_at?: string;
      }>('/api/v1/integrations/task-inbox'),
      save: (email: string, app_password: string, inbox_address: string) =>
        req<{ connected: boolean; email: string; inbox_address: string; allowed_senders: string[] }>(
          '/api/v1/integrations/task-inbox', { method: 'PUT', body: body({ email, app_password, inbox_address }) }),
      setSenders: (allowed_senders: string[]) =>
        req<{ allowed_senders: string[] }>('/api/v1/integrations/task-inbox/senders', {
          method: 'PATCH', body: body({ allowed_senders }) }),
      sync: () => req<SyncResult>('/api/v1/integrations/task-inbox/sync', { method: 'POST' }),
      delete: () => req<void>('/api/v1/integrations/task-inbox', { method: 'DELETE' }),
    },

    emailForward: {
      get: () => req<{
        smtp_enabled: boolean; smtp_address: string; smtp_port: string;
        webhook_enabled: boolean; webhook_url: string;
      }>('/api/v1/integrations/email-forward'),
    },
  },
};

// In Tauri (desktop) mode, use local SQLite only when no server URL is configured.
// When a server URL is set, use httpApi with Bearer token auth.
import { localApi } from './tauri/local-api';

let _api: typeof httpApi | null = null;
export function resetApiResolver() {
  _api = null;
}
function resolveApi(): typeof httpApi {
  if (!_api) {
    if (isTauri() && getServerUrl()) {
      // Desktop with server configured: use HTTP API with Bearer token auth
      _api = httpApi;
    } else if (isTauri()) {
      // Desktop without server: local-only mode (offline)
      _api = localApi as unknown as typeof httpApi;
    } else {
      _api = httpApi;
    }
  }
  return _api;
}

export const api = new Proxy({} as typeof httpApi, {
  get(_target, prop) {
    return (resolveApi() as Record<string | symbol, unknown>)[prop];
  },
});
