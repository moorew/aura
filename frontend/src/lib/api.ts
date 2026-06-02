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
} from './types';

// In dev: set VITE_API_URL=http://localhost:9001. In production (served from Go), leave unset → relative URLs.
const base = (import.meta.env.VITE_API_URL as string | undefined) ?? '';

async function req<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${base}${path}`, {
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include',
    ...init,
  });
  if (!res.ok) {
    const body = await res.text();
    throw new Error(`${res.status} ${res.statusText}: ${body}`);
  }
  if (res.status === 204) return undefined as T;
  return res.json();
}

const body = (data: unknown) => JSON.stringify(data);

export const api = {
  auth: {
    me: () => req<{ authenticated: boolean; auth_enabled: boolean; username?: string }>('/api/v1/auth/me'),
    login: (username: string, password: string) =>
      req<{ status: string }>('/api/v1/auth/login', { method: 'POST', body: body({ username, password }), credentials: 'include' }),
    logout: () => req<void>('/api/v1/auth/logout', { method: 'POST', credentials: 'include' }),
  },

  tasks: {
    listByDate:  (date: string)      => req<Task[]>(`/api/v1/tasks?date=${date}`),
    listByWeek:  (weekStart: string) => req<Task[]>(`/api/v1/tasks?week_start=${weekStart}`),
    listBacklog: ()                  => req<Task[]>('/api/v1/tasks'),
    get:         (id: string)        => req<Task>(`/api/v1/tasks/${id}`),
    create: (input: CreateTaskInput) =>
      req<Task>('/api/v1/tasks', { method: 'POST', body: body(input) }),
    update: (id: string, patch: UpdateTaskInput) =>
      req<Task>(`/api/v1/tasks/${id}`, { method: 'PATCH', body: body(patch) }),
    delete: (id: string) => req<void>(`/api/v1/tasks/${id}`, { method: 'DELETE' }),
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

  integrations: {
    jira: {
      get: () => req<JiraIntegrationConfig>('/api/v1/integrations/jira'),
      save: (cfg: { host: string; email: string; api_token: string; jql?: string }) =>
        req<JiraIntegrationConfig>('/api/v1/integrations/jira', { method: 'PUT', body: body(cfg) }),
      test: () => req<{ status: string }>('/api/v1/integrations/jira/test', { method: 'POST' }),
      sync: () => req<SyncResult>('/api/v1/integrations/jira/sync', { method: 'POST' }),
      delete: () => req<void>('/api/v1/integrations/jira', { method: 'DELETE' }),
    },
    gmail: {
      get: () => req<GmailIntegrationConfig>('/api/v1/integrations/gmail'),
      authUrl: (withCalendar = false) =>
        `${base}/api/v1/integrations/gmail/auth${withCalendar ? '?calendar=1' : ''}`,
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
      toTask: (id: string, subject: string) =>
        req<Task>(`/api/v1/integrations/fastmail/emails/${id}/to-task`, { method: 'POST', body: body({ subject }) }),
      archive: (id: string) =>
        req<void>(`/api/v1/integrations/fastmail/emails/${id}/archive`, { method: 'POST' }),
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
