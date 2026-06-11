/**
 * Notification sound catalogue. The audio files live in static/sounds/<id>.mp3
 * (and are mirrored into the Android res/raw folder for FCM channels). All are
 * short, calm one-shots normalized to a consistent loudness.
 *
 * The `id` doubles as the Android res/raw resource name, so it must stay
 * lowercase with underscores only ([a-z0-9_]).
 */

export interface NotificationSound {
  id: string;
  label: string;
}

export const NOTIFICATION_SOUNDS: NotificationSound[] = [
  { id: 'piano', label: 'Carbon Piano' },
  { id: 'handpan', label: 'Handpan' },
  { id: 'hapi', label: 'Hapi Drum' },
  { id: 'kalimba', label: 'Kalimba' },
  { id: 'pluck', label: 'Fantasy Pluck' },
  { id: 'waterside', label: 'Waterside' },
  { id: 'glimmer', label: 'Glimmer' },
  { id: 'omnidrum', label: 'Omni Drum' },
  { id: 'chord_low', label: 'Low Chord' },
  { id: 'chord_seventh', label: 'Seventh Chord' },
];

export const DEFAULT_SOUND_ID = 'piano';

export function soundUrl(id: string): string {
  return `/sounds/${id}.mp3`;
}

export function soundLabel(id: string): string {
  return NOTIFICATION_SOUNDS.find((s) => s.id === id)?.label ?? id;
}

let current: HTMLAudioElement | null = null;

/**
 * Play a notification sound (preview, or foreground reminder feedback). Stops
 * any sound already playing so rapid previews don't overlap. Best-effort: a
 * blocked autoplay or missing file is swallowed.
 */
export function playSound(id: string): void {
  if (typeof Audio === 'undefined') return;
  try {
    if (current) {
      current.pause();
      current.currentTime = 0;
    }
    const audio = new Audio(soundUrl(id));
    audio.volume = 0.9;
    current = audio;
    void audio.play().catch(() => {});
  } catch {
    /* no-op */
  }
}
