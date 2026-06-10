export type TaskStatus = 'backlog' | 'planned' | 'in_progress' | 'done' | 'cancelled';
export type TaskSource = 'manual' | 'gmail' | 'fastmail' | 'jira' | 'google_calendar';

export interface Task {
  id: string;
  title: string;
  description: string | null;
  planned_date: string | null;
  week_start: string | null;
  status: TaskStatus;
  position: number;
  time_estimate_minutes: number | null;
  time_actual_minutes: number | null;
  parent_task_id: string | null;
  weekly_objective_id: string | null;
  source: TaskSource | null;
  source_id: string | null;
  source_url: string | null;
  source_metadata: string | null;
  completed_at: string | null;
  archived_at: string | null;
  created_at: string;
  updated_at: string;
  // Tags & recurrence
  tags: string[];
  recurrence_rule: string | null;
  recurrence_origin_id: string | null;
  is_customized: boolean;
  // Timeboxing
  scheduled_start: string | null;
  scheduled_end: string | null;
  // "Roughly at" sort hint (HH:MM) — visual ordering only, no time block
  roughly_at: string | null;
}

export interface Attachment {
  id: string;
  owner_type: 'task' | 'objective';
  owner_id: string;
  filename: string;
  mime_type: string;
  size_bytes: number;
  created_at: string;
}

export interface TagDefinition {
  id: string;
  name: string;
  color: string;
  created_at: string;
  updated_at: string;
}

export interface Objective {
  id: string;
  week_start: string;
  title: string;
  description: string | null;
  status: 'active' | 'completed' | 'cancelled';
  position: number;
  created_at: string;
  updated_at: string;
}

export interface DailyPlan {
  id: string;
  plan_date: string;
  status: 'pending' | 'planning' | 'active' | 'shutdown_complete';
  intention: string | null;
  reflection: string | null;
  wins: string | null; // JSON-encoded string[]
  shutdown_at: string | null;
  created_at: string;
  updated_at: string;
}

export interface PomodoroSession {
  id: string;
  task_id: string;
  duration_minutes: number;
  started_at: string;
  completed_at: string | null;
  was_completed: boolean;
  created_at: string;
}

export interface CreateTaskInput {
  title: string;
  description?: string | null;
  planned_date?: string;
  week_start?: string;
  status?: TaskStatus;
  position?: number;
  time_estimate_minutes?: number;
  weekly_objective_id?: string;
  parent_task_id?: string;
  tags?: string[];
  recurrence_rule?: string;
  scheduled_start?: string;
  scheduled_end?: string;
  roughly_at?: string | null;
}

export interface ICalSubscription {
  id: string;
  name: string;
  url: string;
  color: string;
  last_synced_at: string | null;
  error_msg?: string | null;
  created_at: string;
  updated_at: string;
}

export interface ICalEvent {
  id: string;
  subscription_id: string;
  calendar?: string;
  uid: string;
  summary: string;
  description?: string;
  location?: string;
  start_time: string;
  end_time: string;
  all_day: boolean;
  color: string;
}

export interface WeekReview {
  id?: string;
  week_start: string;
  wins: string | null;
  challenges: string | null;
  next_focus: string | null;
  created_at?: string;
  updated_at?: string;
}

export interface UpdateTaskInput {
  title?: string;
  description?: string | null;
  status?: TaskStatus;
  position?: number;
  planned_date?: string | null;
  week_start?: string | null;
  time_estimate_minutes?: number | null;
  time_actual_minutes?: number | null;
  weekly_objective_id?: string | null;
  completed_at?: string | null;
  tags?: string[];
  parent_task_id?: string | null;
  scheduled_start?: string | null;
  scheduled_end?: string | null;
  roughly_at?: string | null;
}

export interface CreateObjectiveInput {
  week_start: string;
  title: string;
  description?: string;
  position?: number;
}

export interface UpdateObjectiveInput {
  title?: string;
  description?: string | null;
  status?: Objective['status'];
  position?: number;
}

export interface UpsertPlanInput {
  status?: DailyPlan['status'];
  intention?: string | null;
  reflection?: string | null;
  wins?: string | null;
  shutdown_at?: string | null;
}

export interface IntegrationConfig {
  connected: boolean;
  enabled?: boolean;
  last_synced_at?: string | null;
}

export interface JiraIntegrationConfig extends IntegrationConfig {
  config?: {
    host: string;
    email: string;
    api_token: string; // empty string when reading back (redacted)
    jql: string;
  };
}

export interface GmailIntegrationConfig extends IntegrationConfig {
  email?: string;
  labels?: string[];
}

export type BackupSecurityMode = 'none' | 'encrypt' | 'exclude_secrets';
export type BackupDestinationType = 'local' | 'webdav' | 's3' | 'drive';

export interface BackupDestination {
  id: string;
  type: BackupDestinationType;
  name: string;
  enabled: boolean;
  // local
  path?: string;
  // webdav
  url?: string;
  username?: string;
  password?: string;
  // s3
  bucket?: string;
  region?: string;
  prefix?: string;
  endpoint?: string;
  access_key_id?: string;
  secret_access_key?: string;
  // drive
  folder_id?: string;
}

export interface BackupSettings {
  enabled: boolean;
  schedule_hour: number;
  retention: number;
  security_mode: BackupSecurityMode;
  has_passphrase: boolean;
  destinations: string; // raw JSON array of BackupDestination
  last_run_at: string | null;
  last_status: string | null;
  last_error: string | null;
  updated_at: string;
}

export interface BackupRun {
  id: string;
  started_at: string;
  finished_at: string | null;
  trigger: string;
  status: 'success' | 'error';
  size_bytes: number | null;
  filename: string | null;
  destinations: string | null; // JSON
  error: string | null;
}

export interface BackupSettingsResponse {
  settings: BackupSettings;
  runs: BackupRun[];
  drive_connected: boolean;
  google_oauth: boolean;
}

export interface SyncResult {
  total: number;
  new: number;
  updated: number;
  errors: number;
}

export const COLUMNS: {
  status: TaskStatus;
  label: string;
  accent: string;
  bg: string;
  border: string;
}[] = [
  { status: 'backlog',     label: 'Triage',  accent: 'bg-slate-400',  bg: 'bg-slate-50 dark:bg-slate-900/50',   border: 'border-slate-200 dark:border-slate-800'  },
  { status: 'planned',     label: 'Planned', accent: 'bg-blue-500',   bg: 'bg-blue-50 dark:bg-blue-950/40',     border: 'border-blue-200 dark:border-blue-900'    },
  { status: 'in_progress', label: 'Focus',   accent: 'bg-amber-500',  bg: 'bg-amber-50 dark:bg-amber-950/40',   border: 'border-amber-200 dark:border-amber-900'  },
  { status: 'done',        label: 'Done',    accent: 'bg-green-500',  bg: 'bg-green-50 dark:bg-green-950/40',   border: 'border-green-200 dark:border-green-900'  },
];

export interface FastmailEmail {
  id: string;
  subject: string;
  from: { name: string; email: string }[];
  received_at: string;
  preview: string;
  is_unread: boolean;
}

// Open Graph / link-preview metadata for a URL, returned by GET /api/v1/unfurl.
// `ok` is false when no usable metadata was found (client falls back to a chip).
export interface LinkUnfurl {
  url: string;
  title: string;
  description: string;
  image_url: string;
  site_name: string;
  favicon_url: string;
  ok: boolean;
  fetched_at: string;
}
