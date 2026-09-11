'use client';

import type { JSX } from 'react';
import type { Task } from '@/entities/task';
import type { IsoDate } from '@/shared/lib/calendar';
import { TaskRow } from '@/features/tasks';
import { propertyDetailTaskTone } from '../lib/detail-tasks';

type PropertyTasksBlockProps = {
  readonly tasks: ReadonlyArray<Task>;
  readonly today: IsoDate;
  readonly canMutate: boolean;
  readonly togglingFor: (task: Task) => boolean;
  readonly onToggle: (task: Task) => void;
  /** Тап по строке; undefined — строка только для чтения. */
  readonly onOpenFor: (task: Task) => (() => void) | undefined;
};

/**
 * Секция «Задачи» детали объекта (тикет #589, правка владельца 11.09,
 * Figma 1185:40816): до 3 канонных строк TaskRow — просрочки от старейшей
 * (красные), затем по ближайшей дате (сегодня/завтра синие, остальные
 * серые), недатированные не выводятся; подпись времени с датой («14
 * сентября, 12:00»), т.к. секций-заголовков здесь нет. Отступ до низа
 * карточки 24 держит обёртка pb-6 (мера — край ряда, как в «Контактах»).
 */
export function PropertyTasksBlock({
  tasks,
  today,
  canMutate,
  togglingFor,
  onToggle,
  onOpenFor,
}: PropertyTasksBlockProps): JSX.Element {
  return (
    <div className="pb-6 pt-1" data-testid="property-tasks-block">
      {tasks.map((task) => (
        <TaskRow
          key={task.id}
          task={task}
          today={today}
          tone={propertyDetailTaskTone(task, today)}
          canMutate={canMutate}
          withDate
          toggling={togglingFor(task)}
          onToggle={() => onToggle(task)}
          onOpen={onOpenFor(task)}
        />
      ))}
    </div>
  );
}
