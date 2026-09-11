'use client';

import type { JSX } from 'react';
import type { PropertyTasksSummary } from '../lib/tasks-summary';

/**
 * Сводка секции «Задачи» (тикет #589): строка «N активных задач» с числом
 * просроченных красным (тон секций просрочки на задачах объекта), тап —
 * в задачи объекта. Макетом кадры карты #583 блок не покрывают —
 * анатомия строки «Управления»/«Контактов», сверить на приёмке.
 */
export function PropertyTasksBlock({
  summary,
  onSelect,
}: {
  readonly summary: PropertyTasksSummary;
  readonly onSelect: () => void;
}): JSX.Element {
  return (
    <div className="px-3 pb-4 pt-4" data-testid="property-tasks-block">
      <button
        type="button"
        onClick={onSelect}
        className="flex h-[52px] w-full cursor-pointer items-center rounded-button px-1 text-left outline-none transition-colors focus-visible:ring-2 focus-visible:ring-primary hover:bg-surface-muted-hover active:bg-surface-muted-hover"
      >
        <span className="flex min-w-0 flex-col justify-center gap-0.5">
          <span className="truncate text-base font-medium text-content">{summary.title}</span>
          {summary.overdueCount > 0 && (
            <span className="truncate text-sm leading-4 text-danger">
              Просрочено: {summary.overdueCount}
            </span>
          )}
        </span>
      </button>
    </div>
  );
}
