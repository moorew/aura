// Format a Date as YYYY-MM-DD using the device's LOCAL timezone. Using
// toISOString() here would convert to UTC, which rolls the date forward (or
// back) for users whose local time differs from UTC — e.g. after ~8pm US
// Eastern it would report "tomorrow" as today.
export function toLocalISODate(d: Date): string {
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, '0');
  const day = String(d.getDate()).padStart(2, '0');
  return `${y}-${m}-${day}`;
}

export function today(): string {
  return toLocalISODate(new Date());
}

export function offsetDate(dateStr: string, delta: number): string {
  const d = new Date(dateStr + 'T00:00:00');
  d.setDate(d.getDate() + delta);
  return toLocalISODate(d);
}

export function formatDate(dateStr: string): string {
  return new Date(dateStr + 'T00:00:00').toLocaleDateString('en-US', {
    weekday: 'long',
    month: 'long',
    day: 'numeric',
    year: 'numeric',
  });
}

export function formatShortDate(dateStr: string): string {
  return new Date(dateStr + 'T00:00:00').toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
  });
}

export function isToday(dateStr: string): boolean {
  return dateStr === today();
}

// Returns the ISO date string of the Monday of the week containing dateStr
export function weekStart(dateStr: string): string {
  const d = new Date(dateStr + 'T00:00:00');
  const day = d.getDay(); // 0 = Sunday
  const offset = day === 0 ? -6 : 1 - day;
  d.setDate(d.getDate() + offset);
  return toLocalISODate(d);
}

export function formatWeekRange(weekStartStr: string): string {
  const start = new Date(weekStartStr + 'T00:00:00');
  const end   = new Date(weekStartStr + 'T00:00:00');
  end.setDate(end.getDate() + 6);

  const year = start.getFullYear();
  const startFmt = start.toLocaleDateString('en-US', { month: 'long', day: 'numeric' });

  if (start.getMonth() === end.getMonth()) {
    return `${startFmt} – ${end.getDate()}, ${year}`;
  }
  const endFmt = end.toLocaleDateString('en-US', { month: 'long', day: 'numeric' });
  return `${startFmt} – ${endFmt}, ${year}`;
}

export function formatMinutes(minutes: number | null): string {
  if (!minutes) return '';
  if (minutes < 60) return `${minutes}m`;
  const h = Math.floor(minutes / 60);
  const m = minutes % 60;
  return m ? `${h}h ${m}m` : `${h}h`;
}

export function appendPosition(positions: number[]): number {
  if (positions.length === 0) return 1000;
  return Math.max(...positions) + 1000;
}

// Insert at index i within a sorted positions array (midpoint between neighbours).
export function insertPosition(positions: number[], i: number): number {
  if (positions.length === 0) return 1000;
  if (i <= 0) return positions[0] - 1000;
  if (i >= positions.length) return positions[positions.length - 1] + 1000;
  return (positions[i - 1] + positions[i]) / 2;
}

// "Roughly at" sort hint (HH:MM) → minutes since midnight; untimed tasks sort
// last (Infinity). Used to order the daily list chronologically without turning
// tasks into rigid calendar blocks.
export function roughlyAtMinutes(t: { roughly_at?: string | null }): number {
  if (!t.roughly_at) return Number.POSITIVE_INFINITY;
  const [h, m] = t.roughly_at.split(':').map((n) => parseInt(n, 10));
  if (Number.isNaN(h)) return Number.POSITIVE_INFINITY;
  return h * 60 + (Number.isNaN(m) ? 0 : m);
}

// Daily-view ordering: timed tasks first in chronological order, then the rest
// by their manual position. Purely visual — does not lock tasks to time blocks.
export function compareTasksForDay(
  a: { roughly_at?: string | null; position: number },
  b: { roughly_at?: string | null; position: number },
): number {
  const ra = roughlyAtMinutes(a);
  const rb = roughlyAtMinutes(b);
  if (ra !== rb) return ra - rb;
  return a.position - b.position;
}

export function parseMinutes(raw: string): number | null {
  const s = raw.trim().toLowerCase();
  if (!s) return null;
  const h = s.match(/^(\d+(?:\.\d+)?)\s*h$/);
  if (h) return Math.round(parseFloat(h[1]) * 60);
  const m = s.match(/^(\d+)\s*m?$/);
  if (m) return parseInt(m[1], 10);
  return null;
}
