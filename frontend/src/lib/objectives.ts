/**
 * Weekly-objective re-planning helpers.
 *
 * "I didn't get to this objective" is the common case at week's end — so rather
 * than retype it next week, carry it forward. Moving an objective shifts its
 * `week_start` to the following Monday and brings its still-open tasks along
 * (shifted +7 days so they land on the same weekday next week). Completed and
 * cancelled tasks stay put as this week's history.
 */

import { api } from '$lib/api';
import { offsetDate, weekStart } from '$lib/utils';
import type { Objective, Task } from '$lib/types';

/** Objectives that haven't been completed yet — the candidates for carry-forward. */
export function unfinishedObjectives(objectives: Objective[]): Objective[] {
  return objectives.filter((o) => o.status !== 'completed' && o.status !== 'cancelled');
}

/**
 * Move one objective (and its open tasks) into next week. Returns the new
 * week_start so callers can report/navigate. Best-effort on the task moves so a
 * single failure doesn't abort the whole carry-forward.
 */
export async function moveObjectiveToNextWeek(obj: Objective, tasks: Task[]): Promise<string> {
  const nextWs = offsetDate(obj.week_start, 7);

  await api.objectives.update(obj.id, { week_start: nextWs, status: 'active' });

  const open = tasks.filter(
    (t) => t.weekly_objective_id === obj.id && t.status !== 'done' && t.status !== 'cancelled',
  );
  await Promise.all(
    open.map((t) => {
      const newPlanned = t.planned_date ? offsetDate(t.planned_date, 7) : null;
      return api.tasks
        .update(t.id, {
          week_start: newPlanned ? weekStart(newPlanned) : nextWs,
          planned_date: newPlanned,
          status: t.status === 'backlog' ? 'planned' : t.status,
        })
        .catch(() => {});
    }),
  );

  return nextWs;
}

/** Carry every unfinished objective of a week forward. Returns how many moved. */
export async function moveAllUnfinishedToNextWeek(
  objectives: Objective[],
  tasks: Task[],
): Promise<number> {
  const targets = unfinishedObjectives(objectives);
  for (const obj of targets) {
    await moveObjectiveToNextWeek(obj, tasks).catch(() => {});
  }
  return targets.length;
}
