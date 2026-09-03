'use client';

import type { JSX, KeyboardEvent } from 'react';
import { Check, ClockSmall, Repeat } from '@/shared/assets/icons';
import { cn } from '@/shared/lib/cn';
import {
  daysOverdue,
  formatCompletedLabel,
  formatOverdueAgo,
  type IsoDate,
  type Task,
} from '@/entities/task';
import { RoundCheckbox } from '@/shared/ui/design';

/** Цвет акцента строки времени: просрочка красным, сегодня/завтра синим,
 * остальные серым (Figma 1531:12784 — варианты Task Button Miss/Today/
 * Default); выполненные строки — серые. */
export type TaskRowTone = 'danger' | 'primary' | 'muted';

const toneClass: Record<TaskRowTone, string> = {
  danger: 'text-danger',
  primary: 'text-primary',
  muted: 'text-content-tertiary',
};

/** Строка задачи (#499, Figma 1532:49657 «Task Button»): кружок-чекбокс +
 * название + комментарий (до 2 строк) + строка времени (часы, время,
 * ↻ повтора). Тап по строке — «Изменить задачу», тап по кружку —
 * выполнить/снять (обратимо); выполненная строка зачёркнута, с отметкой
 * «Выполнена D» и серым временем. У смотрящего кружок — статичный. */
export type TaskRowProps = {
  readonly task: Task;
  readonly today: IsoDate;
  readonly tone: TaskRowTone;
  readonly canMutate: boolean;
  /** Кружок в полёте: чекбокс глушится, чтобы не ушёл дубль-мутации. */
  readonly toggling?: boolean;
  readonly onToggle: () => void;
  /** Тап по строке — правка правила (#502); не передан — строка только
   * для чтения (выполненные — история со снимком, зритель/архив). */
  readonly onOpen?: () => void;
};

export function TaskRow({
  task,
  today,
  tone,
  canMutate,
  toggling = false,
  onToggle,
  onOpen,
}: TaskRowProps): JSX.Element {
  const completed = task.status === 'completed';
  const openable = onOpen !== undefined;
  const timeLabel = timeSubtitle(task, today, tone);

  const handleKeyDown = (event: KeyboardEvent<HTMLDivElement>): void => {
    // Фокус на чекбоксе не должен открывать редактирование: строка
    // реагирует только на собственные Enter/Space.
    if (event.target !== event.currentTarget) {
      return;
    }
    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault();
      onOpen?.();
    }
  };

  // Журнал удалённого правила — только чтение (словарь #494): кружок
  // выполненной задачи без живого правила показывается, но не тапается —
  // сервер всё равно ответил бы контрактным 409.
  const journalLocked = completed && task.ruleId === null;
  const toggleAvailable = canMutate && !journalLocked;

  return (
    <div
      {...(openable
        ? {
            role: 'button',
            tabIndex: 0,
            'aria-label': `Изменить задачу: ${task.title}`,
            onClick: onOpen,
            onKeyDown: handleKeyDown,
          }
        : {})}
      className={cn(
        'flex w-full items-start gap-4 px-6 py-3 text-left font-sans outline-none focus-visible:ring-4 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-white',
        openable && 'cursor-pointer',
      )}
    >
      {toggleAvailable ? (
        <RoundCheckbox
          checked={completed}
          disabled={toggling}
          onCheckedChange={() => onToggle()}
          onClick={(event) => event.stopPropagation()}
          aria-label={completed ? 'Снять выполнение' : 'Выполнить'}
        />
      ) : (
        <span
          aria-hidden
          className={cn(
            'flex h-6 w-6 shrink-0 items-center justify-center rounded-pill border-[1.5px]',
            completed ? 'border-primary bg-primary text-white' : 'border-content-tertiary',
          )}
        >
          {completed && <Check className="h-4 w-4" aria-hidden />}
        </span>
      )}
      <span className="flex min-w-0 flex-1 flex-col gap-1.5">
        <span className={cn('text-base font-medium leading-[18px] text-content', completed && 'line-through')}>
          {task.title}
        </span>
        {task.comment !== null && task.comment !== '' && (
          <span className="text-sm leading-4 text-content-secondary line-clamp-2">{task.comment}</span>
        )}
        {completed && task.completedDate !== null && (
          <span className="text-sm leading-4 text-content">
            {formatCompletedLabel(task.completedDate, today)}
          </span>
        )}
        {(timeLabel !== null || isRecurring(task)) && (
          <span className={cn('flex items-center gap-1.5 text-sm font-medium leading-4', toneClass[tone])}>
            {timeLabel !== null && <ClockSmall className="h-4 w-4 shrink-0" aria-hidden />}
            <span>{timeLabel}</span>
            {isRecurring(task) && <Repeat className="h-4 w-4 shrink-0" aria-hidden />}
          </span>
        )}
      </span>
    </div>
  );
}

function isRecurring(task: Task): boolean {
  return task.repeat !== null && task.repeat !== 'once';
}

/** Подпись времени строки: у просрочки с прошедшими календарными сутками —
 * «N дней назад»; у задачи на весь день просрочка наступает в конце суток,
 * поэтому «0 дней» не бывает. Остальные строки (и просрочка текущих суток)
 * показывают HH:MM; у задачи без времени подписи нет. */
function timeSubtitle(task: Task, today: IsoDate, tone: TaskRowTone): string | null {
  if (task.dueDate === null) {
    return null;
  }
  if (tone === 'danger' && daysOverdue(task.dueDate, today) > 0) {
    return formatOverdueAgo(task.dueDate, today);
  }
  return task.dueTime;
}
