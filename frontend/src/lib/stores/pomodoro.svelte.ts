import { api } from '$lib/api';

type Phase = 'work' | 'short_break' | 'long_break';

const WORK_SECS        = 25 * 60;
const SHORT_BREAK_SECS =  5 * 60;
const LONG_BREAK_SECS  = 15 * 60;

class PomodoroTimer {
  taskId    = $state<string | null>(null);
  taskTitle = $state<string | null>(null);
  phase     = $state<Phase>('work');
  totalSeconds = $state(WORK_SECS);
  remaining    = $state(WORK_SECS);
  isRunning    = $state(false);
  completedToday = $state(0);
  // Notifies listeners (e.g. day page) of a time_actual update
  lastTimeUpdate = $state<{ taskId: string; newActual: number } | null>(null);

  #intervalId:   ReturnType<typeof setInterval> | null = null;
  #sessionStart: string | null = null;
  #initialActual = 0; // time_actual_minutes of task when work session started

  get progressPct(): number {
    return Math.round(((this.totalSeconds - this.remaining) / this.totalSeconds) * 100);
  }
  get display(): string {
    const m = Math.floor(this.remaining / 60).toString().padStart(2, '0');
    const s = (this.remaining % 60).toString().padStart(2, '0');
    return `${m}:${s}`;
  }
  get phaseLabel(): string {
    if (this.phase === 'work')        return 'Focus';
    if (this.phase === 'short_break') return 'Short break';
    return 'Long break';
  }

  start(taskId: string, taskTitle: string, currentActualMinutes = 0) {
    this.#clearInterval();
    this.taskId          = taskId;
    this.taskTitle       = taskTitle;
    this.#initialActual  = currentActualMinutes;
    this.phase           = 'work';
    this.totalSeconds    = WORK_SECS;
    this.remaining       = WORK_SECS;
    this.#sessionStart   = new Date().toISOString();
    this.#resume();
  }

  togglePause() { this.isRunning ? this.#pause() : this.#resume(); }

  stop() {
    this.#pause();
    this.taskId = null; this.taskTitle = null;
    this.phase  = 'work'; this.totalSeconds = WORK_SECS; this.remaining = WORK_SECS;
    this.#sessionStart = null;
  }

  #resume() { this.isRunning = true; this.#intervalId = setInterval(() => this.#tick(), 1000); }
  #pause()  { this.isRunning = false; this.#clearInterval(); }
  #clearInterval() {
    if (this.#intervalId !== null) { clearInterval(this.#intervalId); this.#intervalId = null; }
  }
  #tick() { if (this.remaining > 0) this.remaining--; else void this.#onComplete(); }

  async #onComplete() {
    this.#pause();
    if (this.phase === 'work') {
      const durationMins = Math.round(this.totalSeconds / 60);
      if (this.taskId && this.#sessionStart) {
        try {
          await api.pomodoros.create({
            task_id: this.taskId, duration_minutes: durationMins,
            started_at: this.#sessionStart, completed_at: new Date().toISOString(),
            was_completed: true,
          });
          // Auto-update task's logged time
          const newActual = this.#initialActual + durationMins;
          await api.tasks.update(this.taskId, { time_actual_minutes: newActual });
          this.lastTimeUpdate = { taskId: this.taskId, newActual };
          this.#initialActual = newActual; // next session starts from new total
        } catch { /* non-critical */ }
      }

      this.completedToday++;
      const isLongBreak  = this.completedToday % 4 === 0;
      this.phase        = isLongBreak ? 'long_break' : 'short_break';
      this.totalSeconds = isLongBreak ? LONG_BREAK_SECS : SHORT_BREAK_SECS;
      this.remaining    = this.totalSeconds;
      this.#sessionStart = null;
    } else {
      this.phase = 'work'; this.totalSeconds = WORK_SECS; this.remaining = WORK_SECS;
      this.#sessionStart = new Date().toISOString();
      return;
    }
    this.#resume();
  }
}

export const pomodoro = new PomodoroTimer();
