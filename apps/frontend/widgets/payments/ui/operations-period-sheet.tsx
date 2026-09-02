'use client';

import { useRef, useState, type JSX } from 'react';
import { Cancel } from '@/shared/assets/icons';
import { clientTodayIso } from '@/entities/payment';
import {
  operationsMonthOf,
  operationsPeriodBoundLabel,
  operationsPeriodDraftOf,
  operationsPeriodMonths,
  pickOperationsPeriodDay,
  settledOperationsPeriod,
  type OperationsMonth,
  type OperationsPeriod,
  type OperationsPeriodDraft,
} from '@/features/payments';
import { Button, IconButton, Modal, ModalContent } from '@/shared/ui/design';
import { WEEKDAY_LABELS } from '@/shared/ui/design/month-grid';
import { OperationsRangeCalendarMonth } from './operations-range-calendar';

export type OperationsPeriodSheetProps = {
  readonly open: boolean;
  readonly onOpenChange: (open: boolean) => void;
  /** Применённый период — исходное состояние черновика открытого шита. */
  readonly period: OperationsPeriod;
  readonly onApply: (period: OperationsPeriod) => void;
};

/**
 * Полноэкранный шит «Выберите период» (#477, Figma 1495-64015, 1502-66060,
 * 1502-66758): крестик закрытия, подписи границ «с 1 ноября» / «по
 * 30 ноября», строка дней недели и стек календарных сеток подряд идущих
 * месяцев — границы диапазона синие, недели диапазона под серой
 * подложкой, будущие месяцы приглушены. Кнопка «Выбрать» применяет период,
 * крестик/свайп отбрасывают тапы. Тап до границы задаёт её, правее —
 * завершает диапазон, по завершённому — перезапускает; диапазон без конца
 * сводится к одному дню. Открытие показывает месяц конца выбора — стек
 * тянется вверх в прошлое (Figma 1502-66758: виден декабрь у границы).
 */
export function OperationsPeriodSheet({
  open,
  onOpenChange,
  period,
  onApply,
}: OperationsPeriodSheetProps): JSX.Element {
  const today = clientTodayIso();
  const [draft, setDraft] = useState<OperationsPeriodDraft>(() => operationsPeriodDraftOf(period));
  // Автоскролл выполняется один раз за сессию открытия — ref-callback
  // целевого месяца (см. attachScrollTarget ниже).
  const scrolledRef = useRef(false);

  // Открытие показывает месяц конца выбора — стек тянется вверх в прошлое
  // (Figma 1502-66758: виден декабрь у границы 01.01.2026). Первые кадры
  // анимации открытия vaul сбрасывают прокрутку контейнера, поэтому позиция
  // прикладывается покадрово, пока не удержится сама (потолок — 120 кадров).
  const attachScrollTarget = (node: HTMLElement | null): void => {
    if (node === null || scrolledRef.current) {
      return;
    }
    scrolledRef.current = true;
    let stableFrames = 0;
    let totalFrames = 0;
    // Цель — заголовок месяца полностью в кадре под фиксированной шапкой.
    const step = (): void => {
      totalFrames += 1;
      const drift = Math.round(node.getBoundingClientRect().top) - 64;
      if (Math.abs(drift) > 8) {
        const container = scrollParentOf(node);
        if (container !== null) {
          container.scrollTop += drift;
        }
        stableFrames = 0;
      } else {
        stableFrames += 1;
      }
      if (stableFrames < 5 && totalFrames < 120) {
        requestAnimationFrame(step);
      }
    };
    requestAnimationFrame(step);
  };

  const apply = (): void => {
    onApply(settledOperationsPeriod(draft));
    onOpenChange(false);
  };

  const months = operationsPeriodMonths(draft, today);
  const applied = settledOperationsPeriod(draft);

  return (
    <Modal open={open} onOpenChange={onOpenChange}>
      <ModalContent
        title="Выберите период"
        titleSrOnly
        fullScreen
        footer={
          <Button className="w-full" onClick={apply} aria-label="Выбрать период">
            Выбрать
          </Button>
        }
      >
        <div className="flex items-center justify-between gap-4">
          <IconButton icon={<Cancel />} label="Закрыть" onClick={() => onOpenChange(false)} />
          <span className="text-xl font-semibold leading-6 text-content">Выберите период</span>
          <span className="w-11 shrink-0" aria-hidden />
        </div>

        <div aria-live="polite" className="grid grid-cols-2 gap-2">
          <span className="rounded-card bg-surface-muted px-4 py-3 text-base leading-[18px] text-content">
            с {operationsPeriodBoundLabel(applied.from, today)}
          </span>
          <span className="rounded-card bg-surface-muted px-4 py-3 text-base leading-[18px] text-content">
            по {operationsPeriodBoundLabel(applied.to, today)}
          </span>
        </div>

        <div
          aria-hidden
          className="grid grid-cols-7 gap-0.5 px-4 pt-2 text-center font-sans text-base font-medium leading-[18px] text-content-tertiary"
        >
          {WEEKDAY_LABELS.map((weekday) => (
            <span key={weekday}>{weekday}</span>
          ))}
        </div>

        <div className="flex flex-col gap-8">
          {months.map((month) => {
            const isScrollTarget = monthId(month) === monthId(operationsMonthOf(period.to));
            return (
              <div
                key={`${month.year}-${month.month}`}
                id={monthId(month)}
                ref={isScrollTarget ? attachScrollTarget : undefined}
              >
                <OperationsRangeCalendarMonth
                  month={month}
                  draft={draft}
                  today={today}
                  onPick={(day) => setDraft(pickOperationsPeriodDay(draft, day))}
                />
              </div>
            );
          })}
        </div>
      </ModalContent>
    </Modal>
  );
}

function monthId(month: OperationsMonth): string {
  return `operations-period-month-${month.year}-${String(month.month + 1).padStart(2, '0')}`;
}

/** Ближайший прокручиваемый предок (тело ModalContent). */
function scrollParentOf(node: HTMLElement): HTMLElement | null {
  let parent = node.parentElement;
  while (parent !== null) {
    const overflowY = getComputedStyle(parent).overflowY;
    if (overflowY === 'auto' || overflowY === 'scroll') {
      return parent;
    }
    parent = parent.parentElement;
  }
  return null;
}
