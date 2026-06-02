export function today(): string {
  return new Date().toISOString().split('T')[0];
}

export function offsetDate(dateStr: string, delta: number): string {
  const d = new Date(dateStr + 'T00:00:00');
  d.setDate(d.getDate() + delta);
  return d.toISOString().split('T')[0];
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
  return d.toISOString().split('T')[0];
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

export function parseMinutes(raw: string): number | null {
  const s = raw.trim().toLowerCase();
  if (!s) return null;
  const h = s.match(/^(\d+(?:\.\d+)?)\s*h$/);
  if (h) return Math.round(parseFloat(h[1]) * 60);
  const m = s.match(/^(\d+)\s*m?$/);
  if (m) return parseInt(m[1], 10);
  return null;
}
