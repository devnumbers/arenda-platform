'use client';

import { useEffect, useRef, useState, type JSX } from 'react';
import { ArrowLeft, ChevronDown } from '@/shared/assets/icons';
import type { IsoDate } from '@/entities/task';
import { calendarMonthOf, listCalendarMonths, type CalendarMonthRef } from '@/features/tasks';
import {
  Button,
  CalendarButton,
  IconButton,
  Modal,
  ModalContent,
  MonthYearPicker,
  StickyBottomBar,
  TopNav,
  TopNavTitle,
} from '@/shared/ui/design';
// Математика месяца — прямой импорт модуля shared (общие слои — точки
// входа сами по себе), как в мастере платежей.
import { MONTH_LABELS, daysInMonth, firstWeekdayOfMonth, WEEKDAY_LABELS } from '@/shared/ui/design/month-grid';

/** Полноэкранный пикер даты задачи (#500, Figma 1539-78660): назад, чип
 * «Август 2026 ⌄» (шит-пикер месяца и года — MonthYearPicker), ниже —
 * лента месяцев подряд под одной шапкой дней недели (сетки — геометрия
 * CalendarMonth #459). Сегодня собственника предвыбрано сразу, поэтому
 * «Выбрать» активна без действий (подсказка владельца к #500); дни раньше
 * сегодня недоступны — задним числом правила не создаются (контракт
 * #498). Рендерится только в открытом состоянии — состояние ленты и
 * черновик живут, пока пикер смонтирован.
 *
 * Снятие даты (решение владельца 2026-09-03): повторный тап по выбранному
 * дню опустошает черновик, «Выбрать» остаётся активной и подтверждает
 * «без даты» (onConfirm(null)) — выбор по-прежнему коммитится явной
 * кнопкой, кнопка активна всегда. */

/** Лента: год вперёд от стартового месяца. */
const FEED_MONTHS = 12;

function isoDateOf(year: number, month0: number, day: number): IsoDate {
  return new Date(Date.UTC(year, month0, day)).toISOString().slice(0, 10);
}

export type TaskDatePickerProps = {
  /** «Сегодня» собственника (ADR 0048) — границы доступности и дефолт. */
  readonly today: IsoDate;
  /** Текущая дата формы; null — черновиком становится сегодня. */
  readonly value: IsoDate | null;
  readonly onClose: () => void;
  /** «Выбрать»: коммитит черновик; null — дата снята («без срока»). */
  readonly onConfirm: (date: IsoDate | null) => void;
};

export function TaskDatePicker({
  today,
  value,
  onClose,
  onConfirm,
}: TaskDatePickerProps): JSX.Element {
  const [draft, setDraft] = useState<IsoDate | null>(value ?? today);
  // Лента — год вперёд от сегодня; значение дальше года открывает ленту
  // от себя (прошлое недоступно, назад лента не идёт).
  const [feedStart, setFeedStart] = useState<CalendarMonthRef>(() => {
    const todayMonth = calendarMonthOf(today);
    if (value !== null) {
      const valueMonth = calendarMonthOf(value);
      const monthsBetween =
        (valueMonth.year - todayMonth.year) * 12 + valueMonth.month0 - todayMonth.month0;
      if (monthsBetween >= FEED_MONTHS) {
        return valueMonth;
      }
    }
    return todayMonth;
  });
  const [monthPickerOpen, setMonthPickerOpen] = useState(false);
  const months = listCalendarMonths(
    `${feedStart.year}-${pad2(feedStart.month0 + 1)}-01`,
    FEED_MONTHS,
  );
  const monthRefs = useRef(new Map<string, HTMLElement>());
  const scrollRef = useRef<HTMLDivElement | null>(null);

  // Чип месяца показывает месяц черновика; у пустого — текущий месяц.
  const draftMonth = calendarMonthOf(draft ?? today);

  // Пересборка ленты (прыжок за её пределы) начинается с выбранного месяца
  // — остаточный скролл прошлой ленты сбрасывается.
  useEffect(() => {
    scrollRef.current?.scrollTo({ top: 0 });
  }, [feedStart]);

  // Escape закрывает пикер; при открытом шите месяца его Esc обрабатывает
  // Radix — глобальный слушатель в этот момент глушится.
  useEffect(() => {
    if (monthPickerOpen) {
      return;
    }
    const onKeyDown = (event: KeyboardEvent): void => {
      if (event.key === 'Escape') {
        onClose();
      }
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [monthPickerOpen, onClose]);

  /** Прыжок по чипу: месяц ленты — скроллом, вне ленты — пересборка ленты
   * от выбранного месяца (прошлое недоступно, назад лента не идёт). */
  const jumpTo = (year: number, month0: number): void => {
    const inFeed = months.some((month) => month.year === year && month.month0 === month0);
    if (inFeed) {
      monthRefs.current.get(`${year}-${month0}`)?.scrollIntoView({ block: 'start' });
      return;
    }
    setFeedStart({ year, month0 });
  };

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label="Выбрать дату"
      className="fixed inset-0 z-50 flex flex-col bg-surface font-sans"
    >
      <TopNav
        leading={<IconButton icon={<ArrowLeft />} label="Назад" onClick={onClose} />}
      >
        <TopNavTitle title="Выбрать дату" />
      </TopNav>

      <div className="min-h-0 flex-1 overflow-y-auto desktop:pt-[72px]">
        <div className="mx-auto w-full max-w-[560px] pb-[136px]">
          <div className="px-6 pt-6">
            <button
              type="button"
              onClick={() => setMonthPickerOpen(true)}
              className="inline-flex h-11 cursor-pointer items-center gap-2 rounded-pill bg-surface-muted px-5 text-base font-medium text-content outline-none transition-colors hover:bg-surface-muted-hover active:bg-surface-muted-hover focus-visible:ring-2 focus-visible:ring-primary"
            >
              {MONTH_LABELS[draftMonth.month0]} {draftMonth.year}
              <ChevronDown className="h-6 w-6 text-content-secondary" aria-hidden />
            </button>
          </div>

          {/* Шапка дней недели — одна на ленту (макет 1539-78660); сетки
              месяцев — геометрия CalendarMonth: заголовок вплотную к шапке,
              грид с отступом 16. */}
          <div className="mt-6 grid grid-cols-7 px-5">
            {WEEKDAY_LABELS.map((weekday) => (
              <div
                key={weekday}
                className="flex aspect-square items-center justify-center text-base font-medium leading-[18px] text-content-tertiary"
              >
                {weekday}
              </div>
            ))}
          </div>
          {months.map(({ year, month0 }, index) => {
            const days = daysInMonth(year, month0);
            const leadingBlanks = firstWeekdayOfMonth(year, month0);
            return (
              <section
                key={`${year}-${month0}`}
                ref={(node) => {
                  if (node !== null) {
                    monthRefs.current.set(`${year}-${month0}`, node);
                  } else {
                    monthRefs.current.delete(`${year}-${month0}`);
                  }
                }}
                className={index === 0 ? undefined : 'mt-8'}
              >
                <h3 className="px-6 text-xl font-semibold leading-6 text-content">
                  {MONTH_LABELS[month0]}, {year}
                </h3>
                <div className="mt-4 grid grid-cols-7 gap-0.5 px-4">
                  {Array.from({ length: leadingBlanks }, (_, blank) => (
                    <div key={`blank-${blank}`} aria-hidden className="aspect-square" />
                  ))}
                  {Array.from({ length: days }, (_, day) => {
                    const iso = isoDateOf(year, month0, day + 1);
                    const selected = iso === draft;
                    const isToday = iso === today;
                    return (
                      <CalendarButton
                        key={iso}
                        className="aspect-square h-auto w-full"
                        state={selected ? 'selected' : isToday ? 'today' : 'default'}
                        aria-current={isToday ? 'date' : undefined}
                        disabled={iso < today}
                        // Повторный тап по выбранному дню снимает выбор
                        // (решение владельца 2026-09-03).
                        onClick={() => setDraft(selected ? null : iso)}
                      >
                        {day + 1}
                      </CalendarButton>
                    );
                  })}
                </div>
              </section>
            );
          })}
        </div>
      </div>

      <StickyBottomBar>
        {/* Активна всегда: с пустым черновиком подтверждает «без даты». */}
        <Button className="w-full" onClick={() => onConfirm(draft)}>
          Выбрать
        </Button>
      </StickyBottomBar>

      <Modal open={monthPickerOpen} onOpenChange={setMonthPickerOpen}>
        <ModalContent title="Месяц и год" titleSrOnly>
          <MonthYearPicker
            month={draftMonth.month0}
            year={draftMonth.year}
            onConfirm={(month0, year) => {
              setMonthPickerOpen(false);
              jumpTo(year, month0);
            }}
          />
        </ModalContent>
      </Modal>
    </div>
  );
}

function pad2(value: number): string {
  return String(value).padStart(2, '0');
}
