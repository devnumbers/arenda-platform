'use client';

import type { JSX } from 'react';
import type { OperationsSummary } from '@/entities/payment';
import { summaryBarSegments, SummaryBarStrip } from '@/features/payment-categories';
import { Skeleton } from '@/shared/ui/design';
import { formatMoneyKopecks } from '@/shared/lib/format-money';

function OperationsTotalsGroup({
  label,
  totalKopecks,
  segments,
}: {
  readonly label: string;
  readonly totalKopecks: number | undefined;
  readonly segments: ReturnType<typeof summaryBarSegments>;
}): JSX.Element {
  return (
    <div className="flex min-w-0 flex-1 flex-col gap-3">
      <div className="flex flex-col gap-0.5">
        <span className="text-base font-medium leading-[18px] text-content">
          {totalKopecks === undefined ? '—' : formatMoneyKopecks(totalKopecks)}
        </span>
        <span className="text-sm leading-4 text-content">{label}</span>
      </div>
      <SummaryBarStrip segments={segments} />
    </div>
  );
}

/** Скелетон сводки (§7, приглушённый в серой карточке) — общий для
 * isLoading-ветки блока и страничного скелетона деталей (паритет #604:
 * одна анатомия на обе фазы загрузки). */
export function PropertyOperationsSkeleton(): JSX.Element {
  return (
    <>
      {[0, 1].map((column) => (
        <div key={column} className="flex min-w-0 flex-1 flex-col gap-3">
          <Skeleton className="h-5 w-24 bg-surface-muted-hover" />
          <Skeleton className="h-1.5 w-full bg-surface-muted-hover" />
        </div>
      ))}
    </>
  );
}

/**
 * Сводка «Операции в <месяц>» на детали объекта (тикет #589, Figma
 * 1185:40820): две группы 50/50 — «Расходы» и «Доходы», сумма 16/500 над
 * подписью 14/16, полоса 6px из пилюль категорий (канонный
 * SummaryBarStrip той же разбивки, что на экране операций). Период и
 * итоги — серверная сводка оплаченных операций за текущий месяц. Пока
 * сводка едет — скелетон (§7, приглушённый в серой карточке).
 */
export function PropertyOperationsBlock({
  summary,
  isLoading,
}: {
  readonly summary: OperationsSummary | undefined;
  readonly isLoading: boolean;
}): JSX.Element {
  return (
    <div className="flex items-center gap-5 px-6 pb-6 pt-4" data-testid="property-operations-block">
      {isLoading ? (
        <PropertyOperationsSkeleton />
      ) : (
        <>
          <OperationsTotalsGroup
            label="Расходы"
            totalKopecks={summary?.expenseTotalKopecks}
            segments={summaryBarSegments(summary, 'expense')}
          />
          <OperationsTotalsGroup
            label="Доходы"
            totalKopecks={summary?.incomeTotalKopecks}
            segments={summaryBarSegments(summary, 'income')}
          />
        </>
      )}
    </div>
  );
}
