// Modular, client-side filtering for the Jira sidebar.
//
// Design: each Jira facet (Open, Assigned to me, Priority, Type, Epic, …) is a
// declarative `JiraFilterDef`. The sidebar component renders and applies these
// generically, so adding a new filter later means appending one descriptor to
// `JIRA_FILTERS` — no changes to the component or the apply pipeline.
//
// Filtering runs against the already-synced local tasks (offline-friendly).
// The enabling data lives in `task.source_metadata`, enriched at sync time by
// the Go backend (see backend/internal/integrations/jira/sync.go).

import type { Task } from '$lib/types';

export interface JiraMeta {
  key?: string;
  status?: string;
  /** Jira status category: "new" | "indeterminate" | "done". */
  statusCategory?: string;
  issueType?: string;
  priority?: string;
  /** Assignee display name, when present. */
  assignee?: string;
  /** True when the issue is assigned to the connected Jira account. */
  mine?: boolean;
  /** Parent/epic key + summary (best-effort; team-managed parents). */
  epicKey?: string;
  epicName?: string;
  /** Active sprint name (requires the instance's sprint custom field). */
  sprint?: string;
}

export function parseJiraMeta(raw: string | null): JiraMeta | null {
  if (!raw) return null;
  try {
    return JSON.parse(raw) as JiraMeta;
  } catch {
    return null;
  }
}

// ── Filter descriptors ────────────────────────────────────────────────────────

export type JiraFilterKind = 'toggle' | 'select';

export interface JiraFilterDef {
  id: string;
  label: string;
  kind: JiraFilterKind;
  /** Default state: toggles → on/off; selects → ignored (always start "any"). */
  defaultOn?: boolean;
  /**
   * toggle: predicate that must hold when the toggle is ON. Predicates are
   * written to "fail open": when the underlying metadata is absent (e.g. data
   * synced before enrichment shipped), they keep the issue rather than hide it.
   */
  predicate?: (meta: JiraMeta, task: Task) => boolean;
  /** select: reads this issue's value for the facet (drives options + matching). */
  facetValue?: (meta: JiraMeta, task: Task) => string | undefined;
}

const DONE_NAME = /^(done|closed|resolved|complete|cancelled|canceled)/i;

export const JIRA_FILTERS: JiraFilterDef[] = [
  {
    id: 'open',
    label: 'Open',
    kind: 'toggle',
    defaultOn: true,
    predicate: (m) => {
      if (m.statusCategory) return m.statusCategory !== 'done';
      if (m.status) return !DONE_NAME.test(m.status); // fallback for un-enriched data
      return true;
    },
  },
  {
    id: 'mine',
    label: 'Assigned to me',
    kind: 'toggle',
    defaultOn: true,
    // Exclude only issues KNOWN to belong to someone else. Missing `mine`
    // (un-enriched data, or unassigned) is kept so the filter is safe by default.
    predicate: (m) => m.mine !== false,
  },
  { id: 'priority', label: 'Priority', kind: 'select', facetValue: (m) => m.priority || undefined },
  { id: 'issueType', label: 'Type', kind: 'select', facetValue: (m) => m.issueType || undefined },
  { id: 'status', label: 'Status', kind: 'select', facetValue: (m) => m.status || undefined },
  {
    id: 'epic',
    label: 'Epic',
    kind: 'select',
    facetValue: (m) => m.epicName || m.epicKey || undefined,
  },
  { id: 'sprint', label: 'Sprint', kind: 'select', facetValue: (m) => m.sprint || undefined },
];

export const JIRA_TOGGLE_DEFS = JIRA_FILTERS.filter((d) => d.kind === 'toggle');
export const JIRA_SELECT_DEFS = JIRA_FILTERS.filter((d) => d.kind === 'select');

// ── Filter state ──────────────────────────────────────────────────────────────

export interface JiraFilterState {
  toggles: Record<string, boolean>;
  selects: Record<string, string>; // '' = any
  query: string;
}

export function defaultJiraFilterState(): JiraFilterState {
  const toggles: Record<string, boolean> = {};
  const selects: Record<string, string> = {};
  for (const d of JIRA_FILTERS) {
    if (d.kind === 'toggle') toggles[d.id] = d.defaultOn ?? false;
    else selects[d.id] = '';
  }
  return { toggles, selects, query: '' };
}

/** Count of facet selects that are actively narrowing (non-"any"). */
export function activeSelectCount(state: JiraFilterState): number {
  return JIRA_SELECT_DEFS.reduce((n, d) => n + (state.selects[d.id] ? 1 : 0), 0);
}

/** Distinct, sorted values present in `tasks` for a select facet. */
export function optionsFor(def: JiraFilterDef, tasks: Task[]): string[] {
  const set = new Set<string>();
  for (const t of tasks) {
    const v = def.facetValue?.(parseJiraMeta(t.source_metadata) ?? {}, t);
    if (v) set.add(v);
  }
  return [...set].sort((a, b) => a.localeCompare(b));
}

// ── Fuzzy search ──────────────────────────────────────────────────────────────

/** Substring OR ordered-subsequence match (dependency-free, good enough). */
export function fuzzyMatch(query: string, text: string): boolean {
  if (!query) return true;
  if (text.includes(query)) return true;
  let qi = 0;
  for (let i = 0; i < text.length && qi < query.length; i++) {
    if (text[i] === query[qi]) qi++;
  }
  return qi === query.length;
}

// ── Apply pipeline ────────────────────────────────────────────────────────────

export function applyJiraFilters(tasks: Task[], state: JiraFilterState): Task[] {
  const q = state.query.trim().toLowerCase();
  return tasks.filter((t) => {
    const meta = parseJiraMeta(t.source_metadata) ?? {};
    for (const def of JIRA_FILTERS) {
      if (def.kind === 'toggle') {
        if (state.toggles[def.id] && def.predicate && !def.predicate(meta, t)) return false;
      } else {
        const sel = state.selects[def.id];
        if (sel && def.facetValue?.(meta, t) !== sel) return false;
      }
    }
    if (q) {
      const hay = `${meta.key ?? ''} ${t.title ?? ''}`.toLowerCase();
      if (!fuzzyMatch(q, hay)) return false;
    }
    return true;
  });
}
