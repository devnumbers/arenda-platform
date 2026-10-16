import type { OperationsSummary } from '@/entities/payment';
import type { OperationsCategoryRow } from '@/features/payments';
import { operationsCategoryRows } from '@/features/payments';

import { hasNoPaidOperationsEver } from './operations-empty-states';

/**
 * Общий каркас лент операций (#478): один взгляд на три запроса — лента,
 * периодная сводка и all-time сводка — решает, что показывает экран:
 * скелетон первой загрузки, гейт «Операций еще не было» или разбивку
 * категорий. Инвариант скелетона: pending — только пока данных нет вовсе;
 * ошибка без данных показывает карточку повтора, недогрузка одного из
 * соседей при keepPreviousData — прежние данные, не скелетон. Живёт в
 * widgets/payments: знает про оба слоя сводок, но не про React —
 * структурный шов принимает любые query-результаты с data/isError.
 * Потребители — глобальная лента (#541), направления (#548), лента и
 * направления объекта (#674/#679); поиск — другой каркас (2 запроса +
 * гейт пустого запроса), сознательно не здесь.
 */
type FeedQuery<TData> = {
  readonly data: TData | undefined;
  readonly isError: boolean;
};

export function operationsFeedGate(
  list: FeedQuery<unknown>,
  summary: FeedQuery<OperationsSummary>,
  ever: FeedQuery<OperationsSummary>,
): {
  readonly pending: boolean;
  readonly neverHad: boolean;
  readonly categoryRows: ReadonlyArray<OperationsCategoryRow>;
} {
  const pending =
    (list.data === undefined
      || summary.data === undefined
      || ever.data === undefined)
    && !list.isError
    && !summary.isError
    && !ever.isError;
  return {
    pending,
    neverHad: !list.isError && hasNoPaidOperationsEver(ever.data),
    categoryRows: operationsCategoryRows(summary.data?.categories ?? []),
  };
}
