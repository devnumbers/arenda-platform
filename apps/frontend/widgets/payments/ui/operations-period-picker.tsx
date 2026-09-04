'use client';

import type { JSX } from 'react';
import { clientTodayIso } from '@/entities/payment';
import { defaultOperationsPeriod, useOperationsFilters } from '@/features/payments';
import { CalendarRangePicker } from '@/shared/ui/design';

/** Пикер периода операций — канон CalendarRangePicker поверх списков
 * (решение владельца 2026-09-04, вместо маршрута /operations/period):
 * подтверждение пишет диапазон в адрес списка с заменой записи истории
 * (пикер в ней не остаётся, как прежняя страница периода). Подтверждение
 * неизменённого дефолта не делает период «явным» — чип остаётся
 * «Сентябрь 2026» (период не пишется в URL). */
export function OperationsPeriodPickerDialog({
  onClose,
}: {
  readonly onClose: () => void;
}): JSX.Element {
  const { filters, applyPeriod } = useOperationsFilters();
  const today = clientTodayIso();
  const applied = filters.period ?? defaultOperationsPeriod(today);

  return (
    <CalendarRangePicker
      today={today}
      value={applied}
      onClose={onClose}
      onConfirm={(range) => {
        const stillDefault =
          filters.period === null && range.from === applied.from && range.to === applied.to;
        if (!stillDefault) {
          applyPeriod(range, { replace: true });
        }
        onClose();
      }}
    />
  );
}
