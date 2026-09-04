'use client';

import { useEffect, useRef, useState, type JSX } from 'react';
import { ArrowLeft, ChevronDown } from '@/shared/assets/icons';
import { cn } from '@/shared/lib/cn';
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
// Математика ленты и месяца — прямой импорт модулей shared (общие слои —
// точки входа сами по себе), как в мастере платежей.
import {
  calendarFeedStart,
  calendarMonthIndex,
  calendarMonthOf,
  calendarMonthOfIndex,
  isoDateOf,
  listCalendarMonths,
  type IsoDate,
} from '@/shared/ui/design/calendar-feed';
import { MONTH_LABELS, daysInMonth, firstWeekdayOfMonth, WEEKDAY_LABELS } from '@/shared/ui/design/month-grid';

/** Полноэкранный пикер даты (общий компонент дизайн-слоя, вырос из пикера
 * задач #500, Figma 1539-78660): назад, чип «Август 2026 ⌄» (шит
 * MonthYearPicker) и шапка дней недели закреплены над прокруткой — лента
 * месяцев скроллится под ними (решение владельца 2026-09-04). Лента
 * бесконечна вперёд: старт — самый ранний из «сегодня» и значения (якорь
 * правки #502), месяцы дорисовываются годом при приближении к нижнему
 * краю, а оффскрин-блоки не участвуют в layout (content-visibility) —
 * прокрутка остаётся плавной на любой глубине. Сегодня собственника
 * предвыбрано сразу, поэтому «Выбрать» активна без действий (подсказка
 * владельца к #500); дни раньше сегодня недоступны — задним числом даты
 * не выбираются (контракт #498), кроме текущего значения-якоря в прошлом
 * при правке (#502). Рендерится только в открытом состоянии — состояние
 * ленты и черновик живут, пока пикер смонтирован.
 *
 * Снятие даты (решение владельца 2026-09-03): повторный тап по выбранному
 * дню опустошает черновик, «Выбрать» остаётся активной и подтверждает
 * «без даты» (onConfirm(null)) — выбор по-прежнему коммитится явной
 * кнопкой, кнопка активна всегда. */

/** Стартовая лента и шаг дорисовки — год месяцев. */
const FEED_MONTHS = 12;
/** Порог дорисовки: до нижнего края осталось ~2 месяца прокрутки. */
const APPEND_THRESHOLD_PX = 1000;

function monthKey(year: number, month0: number): string {
  return `${year}-${month0}`;
}

export type CalendarDatePickerProps = {
  readonly title?: string;
  readonly confirmLabel?: string;
  /** «Сегодня» собственника (ADR 0048) — границы доступности и дефолт. */
  readonly today: IsoDate;
  /** Текущая дата; null — черновиком становится сегодня. */
  readonly value: IsoDate | null;
  readonly onClose: () => void;
  /** «Выбрать»: коммитит черновик; null — дата снята («без срока»). */
  readonly onConfirm: (date: IsoDate | null) => void;
};

export function CalendarDatePicker({
  title = 'Выбрать дату',
  confirmLabel = 'Выбрать',
  today,
  value,
  onClose,
  onConfirm,
}: CalendarDatePickerProps): JSX.Element {
  // Лента от самого раннего из сегодня и значения вперёд без конца;
  // значение дальше стартового года открывает ленту, уже дорисованную
  // до него, и скроллит к нему при монтаже.
  const feedStart = calendarFeedStart(today, value);
  const startIndex = calendarMonthIndex(feedStart);
  const valueIndex =
    value !== null ? calendarMonthIndex(calendarMonthOf(value)) - startIndex : -1;
  const [monthsCount, setMonthsCount] = useState(() => Math.max(FEED_MONTHS, valueIndex + 1));
  const [draft, setDraft] = useState<IsoDate | null>(value ?? today);
  const [monthPickerOpen, setMonthPickerOpen] = useState(false);
  const months = listCalendarMonths(feedStart, monthsCount);
  const monthRefs = useRef(new Map<string, HTMLElement>());
  const scrollRef = useRef<HTMLDivElement | null>(null);
  // Прыжок за дорисованный край: лента дорисовывается порцией, цель
  // скроллится после монтирования новых месяцев.
  const pendingScrollIndex = useRef<number | null>(null);

  // Чип месяца показывает месяц черновика; у пустого — текущий месяц.
  const draftMonth = calendarMonthOf(draft ?? today);

  // Значение за пределами стартового года — сразу к нему при открытии.
  useEffect(() => {
    if (valueIndex >= FEED_MONTHS) {
      const target = calendarMonthOfIndex(startIndex + valueIndex);
      monthRefs.current.get(monthKey(target.year, target.month0))?.scrollIntoView({ block: 'start' });
    }
  }, [valueIndex, startIndex]);

  // Дорисовка прыжка за край — после монтирования новых месяцев.
  useEffect(() => {
    const target = pendingScrollIndex.current;
    if (target === null) {
      return;
    }
    pendingScrollIndex.current = null;
    const targetMonth = calendarMonthOfIndex(startIndex + target);
    monthRefs.current.get(monthKey(targetMonth.year, targetMonth.month0))?.scrollIntoView({ block: 'start' });
  }, [monthsCount, startIndex]);

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

  // Приближение к нижнему краю дорисовывает ленту ещё годом — вперёд
  // месяцы есть всегда; DOM растёт только пройденной глубиной, а тяжёлый
  // layout оффскрина снимает content-visibility на блоках месяцев.
  const handleScroll = (): void => {
    const list = scrollRef.current;
    if (list === null) return;
    if (list.scrollTop + list.clientHeight >= list.scrollHeight - APPEND_THRESHOLD_PX) {
      setMonthsCount((count) => count + FEED_MONTHS);
    }
  };

  /** Прыжок по чипу: месяц ленты — скроллом, за дорисованным краем —
   * дорисовка порции и скролл после монтирования (назад лента не идёт). */
  const jumpTo = (year: number, month0: number): void => {
    const target = calendarMonthIndex({ year, month0 }) - startIndex;
    if (target < monthsCount) {
      monthRefs.current.get(monthKey(year, month0))?.scrollIntoView({ block: 'start' });
      return;
    }
    pendingScrollIndex.current = target;
    setMonthsCount(target + FEED_MONTHS);
  };

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label={title}
      className="fixed inset-0 z-50 flex flex-col bg-surface font-sans"
    >
      <TopNav
        leading={<IconButton icon={<ArrowLeft />} label="Назад" onClick={onClose} />}
      >
        <TopNavTitle title={title} />
      </TopNav>

      {/* Закреплённая шапка: чип месяца и строка дней недели над прокруткой
          (требование владельца к бесконечному календарю); на десктопе TopNav
          зафиксирован над экраном — шапка встаёт под ним, лента скроллится
          между шапкой и нижней панелью. */}
      <div className="shrink-0 desktop:mt-[72px]">
        <div className="mx-auto w-full max-w-[560px]">
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
        </div>
      </div>

      <div
        ref={scrollRef}
        onScroll={handleScroll}
        className="min-h-0 flex-1 overflow-y-auto"
      >
        <div className="mx-auto w-full max-w-[560px] pb-[136px]">
          {months.map(({ year, month0 }, index) => {
            const days = daysInMonth(year, month0);
            const leadingBlanks = firstWeekdayOfMonth(year, month0);
            return (
              <section
                key={monthKey(year, month0)}
                ref={(node) => {
                  if (node !== null) {
                    monthRefs.current.set(monthKey(year, month0), node);
                  } else {
                    monthRefs.current.delete(monthKey(year, month0));
                  }
                }}
                // Оффскрин-блок без layout/paint: оценка 480px до первого
                // прохода, дальше браузер держит реальный размер.
                className={cn(
                  index === 0 ? undefined : 'mt-8',
                  '[content-visibility:auto] [contain-intrinsic-size:auto_480px]',
                )}
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
                        // Прошлые дни недоступны — задним числом даты не
                        // создаются (контракт #498); исключение — текущее
                        // значение-якорь в прошлом (правка #502): оно остаётся
                        // тапабельным, повторный тап снимает дату.
                        disabled={iso < today && iso !== value}
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
          {confirmLabel}
        </Button>
      </StickyBottomBar>

      <Modal open={monthPickerOpen} onOpenChange={setMonthPickerOpen}>
        <ModalContent title="Месяц и год" titleSrOnly>
          <MonthYearPicker
            month={draftMonth.month0}
            year={draftMonth.year}
            min={feedStart}
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
