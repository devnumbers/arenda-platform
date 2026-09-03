'use client';

import { useLayoutEffect, useRef, useState, type JSX } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import { Cancel } from '@/shared/assets/icons';
import { goBack } from '@/shared/lib/navigation';
import { clientTodayIso } from '@/entities/payment';
import {
  defaultOperationsPeriod,
  operationsMonthIndex,
  operationsMonthOf,
  operationsPeriodBoundLabel,
  operationsPeriodDraftOf,
  pickOperationsPeriodDay,
  readOperationsFilters,
  resolveFilterReturnPath,
  settledOperationsPeriod,
  shiftOperationsMonth,
  type OperationsMonth,
  type OperationsPeriodDraft,
} from '@/features/payments';
import { Button, IconButton, PageContent, StickyBottomBar, TopNav, TopNavTitle } from '@/shared/ui/design';
import { WEEKDAY_LABELS } from '@/shared/ui/design/month-grid';
import { OperationsRangeCalendarMonth } from './operations-range-calendar';

/** Сколько месяцев дорисовывается за одно дотягивание к верху стека. */
const PREPEND_CHUNK = 6;

/** Порог от верха контейнера, на котором дорисовывается следующая порция. */
const PREPEND_THRESHOLD_PX = 32;

export type OperationsPeriodScreenProps = {
  readonly propertyId: string;
};

/**
 * Страница «Выберите период» (#477, Figma 1495-64015, 1502-65940,
 * 1502-66758) — не попап, а отдельный маршрут. Верх закреплён целиком:
 * крестик + заголовок, поля границ «с … / по …» и строка дней недели —
 * выбор всегда перед глазами. Под ним бесконечный календарь: месяцы
 * дорисовываются при прокрутке вверх без нижнего предела, вниз — текущий
 * месяц плюс два приглушённых будущих. «Выбрать» — закреплённая нижняя
 * панель: применяет период возвратом на список (router.replace — страница
 * выбора не остаётся в истории), крестик — goBack без изменений. Подтверждение
 * неизменённого дефолта не делает период «явным» — чип остаётся «Сентябрь
 * 2026». Тап до границы задаёт её, правее — завершает диапазон, по
 * завершённому — перезапускает; диапазон без конца сводится к одному дню.
 */
export function OperationsPeriodScreen({
  propertyId,
}: OperationsPeriodScreenProps): JSX.Element {
  const router = useRouter();
  const searchParams = useSearchParams();

  const today = clientTodayIso();
  const filters = readOperationsFilters(searchParams, today);
  const applied = filters.period ?? defaultOperationsPeriod(today);
  const [draft, setDraft] = useState<OperationsPeriodDraft>(() =>
    operationsPeriodDraftOf(applied),
  );

  // Куда возвращаться: список, открывший страницу (только маршруты операций
  // этого объекта), иначе — главный список.
  const returnTo = resolveFilterReturnPath(searchParams.get('return'), propertyId);

  // Бесконечный стек: первый отрисованный месяц; при дотягивании к верху
  // сдвигается в прошлое порциями. Старт — от старта выбора (но не глубже
  // пяти месяцев до текущего), низ — текущий месяц + 2 будущих.
  const currentMonth = operationsMonthOf(today);
  const [firstMonth, setFirstMonth] = useState<OperationsMonth>(() => {
    const startIdx = Math.min(
      operationsMonthIndex(operationsMonthOf(applied.from)) - 1,
      operationsMonthIndex(currentMonth) - 5,
    );
    return { year: Math.floor(startIdx / 12), month: startIdx % 12 };
  });
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const anchorRef = useRef<number | null>(null);
  const scrolledRef = useRef(false);

  // Дорисовка порций сдвигает контент вниз — возвращаем прокрутку на место.
  useLayoutEffect(() => {
    const container = scrollRef.current;
    if (anchorRef.current === null || container === null) {
      return;
    }
    container.scrollTop += container.scrollHeight - anchorRef.current;
    anchorRef.current = null;
  });

  const handleScroll = (): void => {
    const container = scrollRef.current;
    if (container === null || anchorRef.current !== null) {
      return;
    }
    if (container.scrollTop > PREPEND_THRESHOLD_PX) {
      return;
    }
    anchorRef.current = container.scrollHeight;
    setFirstMonth((month) => shiftOperationsMonth(month, -PREPEND_CHUNK));
  };

  const targetMonth = operationsMonthOf(applied.to);
  const attachTarget = (node: HTMLDivElement | null): void => {
    if (node !== null && !scrolledRef.current) {
      scrolledRef.current = true;
      node.scrollIntoView({ block: 'start' });
    }
  };

  const cancel = (): void => {
    const params = new URLSearchParams(searchParams);
    params.delete('return');
    const queryString = params.toString();
    goBack(router, `${returnTo}${queryString ? `?${queryString}` : ''}`);
  };

  const apply = (): void => {
    const settled = settledOperationsPeriod(draft);
    // Подтверждение неизменённого дефолта не делает его «явным»: период,
    // пришедший из URL, сохраняется как был.
    const explicit =
      filters.period !== null || settled.from !== applied.from || settled.to !== applied.to;
    const query = new URLSearchParams();
    if (explicit) {
      query.set('from', settled.from);
      query.set('to', settled.to);
    }
    if (filters.categories.length > 0) {
      query.set('category', filters.categories.join(','));
    }
    const queryString = query.toString();
    router.replace(`${returnTo}${queryString ? `?${queryString}` : ''}`);
  };

  const lastMonth = shiftOperationsMonth(currentMonth, 2);
  const months = monthsFromTo(firstMonth, lastMonth);
  // Поля границ следуют за тапами черновика (Figma 1502-66060: «с 10
  // октября / по 19 ноября» прямо во время выбора), а не за применённым.
  const shown = settledOperationsPeriod(draft);

  return (
    <>
      {/* Каркас — как у всех страниц: TopNav (на десктопе — с «крыльями»
       * лого/профиль, закреплён наверху) + PageContent 560 + StickyBottomBar.
       * Верх блока выбора закреплён: заголовок в TopNav, поля границ и дни
       * недели — шапка колонки (Figma 1495-64165: поля 24 по бокам, от
       * хедера 24, зазор до дней недели 8, плашки дней 48px). Колонка во
       * всю высоту вьюпорта минус TopNav — календарь скроллится внутри
       * себя, страница остаётся на месте. */}
      <TopNav
        leading={<IconButton icon={<Cancel />} label="Закрыть" onClick={cancel} />}
      >
        <TopNavTitle title="Выберите период" />
      </TopNav>

      <PageContent className="flex h-[calc(100dvh-72px)] flex-col pb-0">
        <div aria-live="polite" className="grid shrink-0 grid-cols-2 gap-2 px-6">
          <span className="flex h-12 items-center rounded-2xl bg-surface-muted px-4 text-base font-medium leading-[18px] text-content">
            с {operationsPeriodBoundLabel(shown.from, today)}
          </span>
          <span className="flex h-12 items-center rounded-2xl bg-surface-muted px-4 text-base font-medium leading-[18px] text-content">
            по {operationsPeriodBoundLabel(shown.to, today)}
          </span>
        </div>

        <div
          aria-hidden
          className="mt-2 grid shrink-0 grid-cols-7 gap-0.5 px-5 text-center text-base font-medium leading-[18px] text-content-tertiary"
        >
          {WEEKDAY_LABELS.map((weekday) => (
            <span key={weekday} className="flex h-12 items-center justify-center">
              {weekday}
            </span>
          ))}
        </div>

        <div
          ref={scrollRef}
          onScroll={handleScroll}
          className="min-h-0 flex-1 overflow-y-auto pb-[calc(6.5rem+env(safe-area-inset-bottom))]"
        >
          {months.map((month) => (
            <div
              key={`${month.year}-${month.month}`}
              id={`operations-period-month-${month.year}-${String(month.month + 1).padStart(2, '0')}`}
              ref={
                month.year === targetMonth.year && month.month === targetMonth.month
                  ? attachTarget
                  : undefined
              }
            >
              <OperationsRangeCalendarMonth
                month={month}
                draft={draft}
                today={today}
                onPick={(day) => setDraft(pickOperationsPeriodDay(draft, day))}
              />
            </div>
          ))}
        </div>
      </PageContent>

      <StickyBottomBar>
        <Button className="w-full" onClick={apply} aria-label="Выбрать период">
          Выбрать
        </Button>
      </StickyBottomBar>
    </>
  );
}

function monthsFromTo(first: OperationsMonth, last: OperationsMonth): ReadonlyArray<OperationsMonth> {
  const length = operationsMonthIndex(last) - operationsMonthIndex(first) + 1;
  return Array.from({ length }, (_, index) => shiftOperationsMonth(first, index));
}
