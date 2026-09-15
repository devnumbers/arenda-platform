'use client';

import type { JSX } from 'react';
import { clientTodayIso } from '@/entities/payment';
import {
  type OperationsPeriod,
  useGlobalOperationsFilters,
  useOperationsFilters,
} from '@/features/payments';
import { CalendarRangePicker } from '@/shared/ui/design';

/** Тело пикера периода — канон CalendarRangePicker поверх списков (решение
 * владельца 2026-09-04): подтверждение пишет диапазон с заменой записи
 * истории (пикер в ней не остаётся), пустой старт и «Сбросить» — канон
 * #670: без применённого периода открывается ничего не предвыбранным,
 * «Сбросить» возвращает к дефолту (null — from/to уходят из адреса). Чип
 * прыжка «Месяц Год ⌄» скрыт (решение владельца 2026-09-05) — вглубь
 * прошлого ведёт прокрутка ленты с дорисовкой. Состояние передаёт
 * вызывающий хук — объектный или глобальный. */
function PeriodPickerDialogBody({
  period,
  applyPeriod,
  onClose,
}: {
  readonly period: OperationsPeriod | null;
  readonly applyPeriod: (
    period: OperationsPeriod | null,
    options?: { readonly replace?: boolean },
  ) => void;
  readonly onClose: () => void;
}): JSX.Element {
  const today = clientTodayIso();

  return (
    <CalendarRangePicker
      today={today}
      value={period}
      monthJump={false}
      onClose={onClose}
      onConfirm={(range) => {
        applyPeriod(range, { replace: true });
        onClose();
      }}
      onReset={() => {
        applyPeriod(null, { replace: true });
        onClose();
      }}
    />
  );
}

/** Пикер периода объектных экранов. Пустой старт и «Сбросить» — как в
 * глобальном (#674, канон #670); диалог разделяют все объектные экраны
 * операций — экраны операций/направлений объекта на дефолт «весь период»
 * переведены (#674/#675), категории наследуют в своём тикете (#676). */
export function OperationsPeriodPickerDialog({
  onClose,
}: {
  readonly onClose: () => void;
}): JSX.Element {
  const { filters, applyPeriod } = useOperationsFilters();

  return (
    <PeriodPickerDialogBody
      period={filters.period}
      applyPeriod={applyPeriod}
      onClose={onClose}
    />
  );
}

/** Пикер периода глобальной ленты «Операции» (#541, дефолт «весь период»
 * #670): открывается пустым, пока период не применён; любой подтверждённый
 * диапазон пишется в URL с заменой записи истории, «Сбросить» возвращает к
 * «всему периоду» (from/to уходят из адреса). Прочие фильтры (объекты,
 * категории) не трогаются. */
export function OperationsGlobalPeriodPickerDialog({
  onClose,
}: {
  readonly onClose: () => void;
}): JSX.Element {
  const { filters, applyPeriod } = useGlobalOperationsFilters();

  return (
    <PeriodPickerDialogBody
      period={filters.period}
      applyPeriod={applyPeriod}
      onClose={onClose}
    />
  );
}
