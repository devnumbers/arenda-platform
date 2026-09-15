'use client';

import {
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type JSX,
} from 'react';
import { ArrowLeft, SmallArrowDown } from '@/shared/assets/icons';
import { cn } from '@/shared/lib/cn';
import {
  Button,
  CalendarButton,
  IconButton,
  MonthYearPicker,
  StickyBottomBar,
  TopNav,
  TopNavTitle,
} from '@/shared/ui/design';
// Математика ленты и месяца — прямой импорт модулей shared (общие слои —
// точки входа сами по себе), как в мастере платежей.
import {
  booleanRunSegments,
  calendarFeedStart,
  calendarMonthIndex,
  calendarMonthOf,
  calendarMonthOfIndex,
  type CalendarMonthRef,
  type IsoDate,
  type IsoRange,
  type IsoRangeDraft,
  isoDateOf,
  isoDayOfMonth,
  listCalendarMonths,
  pickIsoRange,
  rangeFeedWindow,
  settleIsoRange,
} from '@/shared/lib/calendar';
import { formatRangeBound } from '@/shared/lib/date-format';
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
 * кнопкой, кнопка активна всегда.
 *
 * Обязательная дата (проп required, решение владельца 2026-09-04): у
 * правила «без даты» не существует — повторный тап по выбранному дню
 * выбор не снимает, «Выбрать» с пустым черновиком не подтверждает.
 *
 * Кнопки действия пикера скрыты, пока действия нет (решение владельца
 * 2026-09-05, визард аренды): «Выбрать» с пустым черновиком обязательной
 * даты не рисуется вместе с панелью — вместо погашенной кнопки.
 *
 * Минимальная дата (проп minDate, решение владельца 2026-09-05, визард
 * аренды): minDate — первый доступный день, дни раньше недоступны —
 * окончание аренды строго позже начала (ADR 0053, потребитель передаёт
 * начало + 1); значение раньше минимума в пикер не попадает — потребитель
 * чистит его при смене начала. Пустой черновик стартует с первого
 * доступного дня (сегодня либо minDate, если он позже).
 *
 * Максимальная дата (проп maxDate, #534): дни позже недоступны — дата
 * завершения аренды не бывает будущей (ADR 0053 §3, потребитель передаёт
 * «сегодня» собственника). Симметрично минимуму: пустой черновик
 * прижимается к границе, если «сегодня» за ней.
 *
 * Планшет (561–768, решение владельца 2026-09-04): TopNav и белый шит
 * футера тянутся во всю ширину — кнопка «Выбрать» тоже (StickyBottomBar
 * fullWidthContent); на десктопе ≥769 футер возвращается в колонку 560. */

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
  /** Дата обязательна: пустой черновик «Выбрать» не подтверждает. */
  readonly required?: boolean;
  /** Первый доступный день; дни раньше недоступны (окончание аренды —
   * строго позже начала). */
  readonly minDate?: IsoDate;
  /** Последний доступный день; дни позже недоступны (дата завершения
   * аренды — не будущее, ADR 0053 §3). */
  readonly maxDate?: IsoDate;
  readonly onClose: () => void;
  /** «Выбрать»: коммитит черновик; null — дата снята («без срока»). */
  readonly onConfirm: (date: IsoDate | null) => void;
};

/** Пустой черновик стартует с первого доступного дня: сегодня, прижатое в
 * границы [minDate, maxDate], если «сегодня» за ними. */
function initialDraft(
  today: IsoDate,
  minDate: IsoDate | undefined,
  maxDate: IsoDate | undefined,
): IsoDate {
  let draft = today;
  if (maxDate !== undefined && draft > maxDate) {
    draft = maxDate;
  }
  if (minDate !== undefined && draft < minDate) {
    draft = minDate;
  }
  return draft;
}

export function CalendarDatePicker({
  title = 'Выбрать дату',
  confirmLabel = 'Выбрать',
  today,
  value,
  required,
  minDate,
  maxDate,
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
  const [draft, setDraft] = useState<IsoDate | null>(
    value ?? initialDraft(today, minDate, maxDate),
  );
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
      // Esc, поглощённый шитом месяца, доходит сюда после синхронного
      // закрытия шита: слушатель перерегистрируется в том же диспатче, а
      // Radix помечает событие preventDefault — такой Esc пикер не закрывает.
      if (event.defaultPrevented || event.key !== 'Escape') {
        return;
      }
      onClose();
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
      <div className="shrink-0 tablet:mt-[72px]">
        <div className="mx-auto w-full max-w-[560px]">
          <div className="px-6 pt-6">
            <MonthJumpChip
              label={`${MONTH_LABELS[draftMonth.month0]} ${draftMonth.year}`}
              onClick={() => setMonthPickerOpen(true)}
            />
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
                        // тапабельным, повторный тап снимает дату. Дни раньше
                        // minDate недоступны без исключений (minDate — первый
                        // доступный: окончание аренды строго позже начала,
                        // ADR 0053); дни позже maxDate — тоже (дата
                        // завершения не бывает будущей, #534).
                        disabled={
                          (iso < today && iso !== value)
                          || (minDate !== undefined && iso < minDate)
                          || (maxDate !== undefined && iso > maxDate)
                        }
                        // Повторный тап по выбранному дню снимает выбор
                        // (решение владельца 2026-09-03); с required дата
                        // обязательна — тап её держит.
                        onClick={() => setDraft(selected && !required ? null : iso)}
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

      {/* Кнопка скрыта, пока нечего подтвердить (обязательная дата без
          черновика) — решение владельца 2026-09-05: скрытие вместо
          погашенной кнопки, у пикерных «Выбрать» так же. Без required
          активна всегда: с пустым черновиком подтверждает «без даты».
          Футер тянется вместе с шитом в планшетном диапазоне 561–768
          (решение владельца 2026-09-04: хедер и шит на планшете во всю
          ширину — кнопка тоже). */}
      {!(required === true && draft === null) && (
        <StickyBottomBar fullWidthContent>
          <Button className="w-full" onClick={() => onConfirm(draft)}>
            {confirmLabel}
          </Button>
        </StickyBottomBar>
      )}

      {/* Шит месяца и года — самостоятельный WheelPickerSheet (общий
          компонент), монтируется только в открытом состоянии. */}
      <MonthYearPicker
        open={monthPickerOpen}
        onOpenChange={setMonthPickerOpen}
        month={draftMonth.month0}
        year={draftMonth.year}
        min={feedStart}
        onConfirm={(month0, year) => {
          setMonthPickerOpen(false);
          jumpTo(year, month0);
        }}
      />
    </div>
  );
}


/** Чип «Месяц Год ⌄» прыжка по ленте — общий для обоих пикеров. */
function MonthJumpChip({
  label,
  onClick,
}: {
  readonly label: string;
  readonly onClick: () => void;
}): JSX.Element {
  return (
    <button
      type="button"
      onClick={onClick}
      className="inline-flex h-11 cursor-pointer items-center gap-2 rounded-pill bg-surface-muted px-5 text-base font-medium text-content outline-none transition-colors hover:bg-surface-muted-hover active:bg-surface-muted-hover focus-visible:ring-2 focus-visible:ring-primary"
    >
      {label}
      <SmallArrowDown className="h-6 w-6 text-content-secondary" aria-hidden />
    </button>
  );
}

/** Порог дорисовки назад: до верха контейнера осталось меньше этого. */
const RANGE_PREPEND_THRESHOLD_PX = 32;
/** Сколько месяцев дорисовывается назад за одно дотягивание к верху. */
const RANGE_PREPEND_CHUNK = 6;

export type CalendarRangePickerProps = {
  readonly title?: string;
  readonly confirmLabel?: string;
  /** «Сегодня» собственника (ADR 0048) — граница доступных дней. */
  readonly today: IsoDate;
  /** Применённый диапазон — преселект и стартовое окно ленты; null или
   * не задан — пустой старт: ничего не выбрано, диапазон строится тапами
   * (дефолт «весь период» фильтра операций, #670). */
  readonly value?: IsoRange | null;
  /** Чип «Месяц Год ⌄» прыжка по ленте (по умолчанию показан). На фильтре
   * периода операций скрыт (решение владельца 2026-09-05): путь вглубь
   * прошлого — прокрутка с дорисовкой. */
  readonly monthJump?: boolean;
  readonly onClose: () => void;
  /** «Выбрать»: коммитит завершённый диапазон (неполный — один день). */
  readonly onConfirm: (range: IsoRange) => void;
  /** «Сбросить» рядом с «Выбрать» (#670): возврат к состоянию «без
   * периода». Без пропа кнопки нет — прежним потребителям пикер не меняется. */
  readonly onReset?: () => void;
};

/** Полноэкранный пикер диапазона дат — канон периода операций (решение
 * владельца 2026-09-04: фильтр «Период» переведён с самописной страницы
 * #477 на общий компонент; визуал шапки и ячеек — 1:1 с ним, Figma
 * 1495-64015, 1502-65940/66060/66758, 1495-64165). Отличия от одиночного
 * пикера продиктованы операционной историей: лента бесконечна назад —
 * месяцы дорисовываются порциями при дотягивании к верху (позиция
 * прокрутки сохраняется якорем), вперёд — только текущий месяц с двумя
 * приглушёнными будущими; дни позже «сегодня» недоступны (операции —
 * только paid, резолюция #474). Тап задаёт границу, второй тап завершает
 * диапазон в любую сторону (5→1 = 1–5, решение владельца 2026-09-05), по
 * завершённому — перезапуск; серые «пилюли»
 * подчёркивают недели диапазона, границы — синие ячейки. Поля «с …/по …»
 * над сеткой следуют за тапами вживую (Figma 1502-66060). Кнопка выхода —
 * «Назад» (иконка канона) вместо креста «Закрыть» прежней страницы. Чип
 * «Месяц Год ⌄» прыжка по ленте опционален (monthJump, по умолчанию
 * включён): колесо годов уходит в прошлое без предела (onNearStart),
 * будущее закрыто границей max; на фильтре периода операций чип скрыт
 * (решение владельца 2026-09-05) — вглубь прошлого ведёт прокрутка с
 * дорисовкой. Пустой старт (value не задан, #670 — дефолт «весь период»
 * фильтра операций): ничего не предвыбрано, поля границ — плейсхолдеры,
 * диапазон строится тапами, «Выбрать» активна только при выборе; лента
 * открывается пятью месяцами контекста у текущего. Опциональный onReset
 * добавляет «Сбросить» рядом с «Выбрать» (secondary слева) — потребитель
 * решает, что сброс означает. Футер на планшете тянется вместе с шитом
 * (fullWidthContent). Рендерится только в открытом состоянии — лента и
 * черновик живут, пока пикер смонтирован. */
export function CalendarRangePicker({
  title = 'Выберите период',
  confirmLabel = 'Выбрать',
  today,
  value = null,
  monthJump = true,
  onClose,
  onConfirm,
  onReset,
}: CalendarRangePickerProps): JSX.Element {
  // Черновик открывается по применённому диапазону; без него (#670) —
  // пустой: ничего не выбрано, первый тап задаёт границу.
  const [draft, setDraft] = useState<IsoRangeDraft | null>(() =>
    value !== null ? { start: value.from, end: value.to } : null,
  );
  const shown = draft !== null ? settleIsoRange(draft) : null;
  const draftMonth = calendarMonthOf(shown !== null ? shown.from : today);

  // Окно ленты: стартовое — по rangeFeedWindow, назад дорисовывается
  // порциями без предела, вперёд — жёсткий край у приглушённого будущего.
  const feedWindow = rangeFeedWindow(value, today);
  const [firstMonth, setFirstMonth] = useState<CalendarMonthRef>(feedWindow.first);
  const months = listCalendarMonths(
    firstMonth,
    calendarMonthIndex(feedWindow.last) - calendarMonthIndex(firstMonth) + 1,
  );
  const todayIdx = calendarMonthIndex(calendarMonthOf(today));

  const monthRefs = useRef(new Map<string, HTMLElement>());
  const scrollRef = useRef<HTMLDivElement | null>(null);
  // Высота стека до дорисовки: прокрутка возвращается на место, чтобы
  // prepend не дёргал экран (тот же приём, что на странице периода).
  const anchorRef = useRef<number | null>(null);
  // Месяц прыжка чипа за дорисованный назад край — скроллится после
  // монтирования новых месяцев.
  const pendingJumpRef = useRef<string | undefined>(undefined);
  // Первый рендер — сразу к месяцу конца применённого диапазона.
  const scrolledRef = useRef(false);

  useLayoutEffect(() => {
    const container = scrollRef.current;
    if (container !== null && anchorRef.current !== null) {
      container.scrollTop += container.scrollHeight - anchorRef.current;
      anchorRef.current = null;
    }
    const jump = pendingJumpRef.current;
    if (jump !== undefined) {
      pendingJumpRef.current = undefined;
      monthRefs.current.get(jump)?.scrollIntoView({ block: 'start' });
    }
  });

  const handleScroll = (): void => {
    const container = scrollRef.current;
    if (container === null || anchorRef.current !== null) {
      return;
    }
    if (container.scrollTop > RANGE_PREPEND_THRESHOLD_PX) {
      return;
    }
    anchorRef.current = container.scrollHeight;
    setFirstMonth((month) =>
      calendarMonthOfIndex(calendarMonthIndex(month) - RANGE_PREPEND_CHUNK),
    );
  };

  /** Прыжок по чипу: месяц ленты — скроллом; глубже дорисованного края —
   * дорисовка порциями назад и скролл после монтирования. */
  const jumpTo = (year: number, month0: number): void => {
    const targetIdx = calendarMonthIndex({ year, month0 });
    if (targetIdx >= calendarMonthIndex(firstMonth)) {
      monthRefs.current.get(monthKey(year, month0))?.scrollIntoView({ block: 'start' });
      return;
    }
    let nextFirst = calendarMonthIndex(firstMonth);
    while (nextFirst > targetIdx) {
      nextFirst -= RANGE_PREPEND_CHUNK;
    }
    pendingJumpRef.current = monthKey(year, month0);
    setFirstMonth(calendarMonthOfIndex(nextFirst));
  };

  // Escape закрывает пикер; при открытом шите месяца его Esc обрабатывает
  // Radix — глобальный слушатель в этот момент глушится.
  const [monthPickerOpen, setMonthPickerOpen] = useState(false);
  useEffect(() => {
    if (monthPickerOpen) {
      return;
    }
    const onKeyDown = (event: KeyboardEvent): void => {
      // Esc, поглощённый шитом месяца, доходит сюда после синхронного
      // закрытия шита: слушатель перерегистрируется в том же диспатче, а
      // Radix помечает событие preventDefault — такой Esc пикер не закрывает.
      if (event.defaultPrevented || event.key !== 'Escape') {
        return;
      }
      onClose();
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [monthPickerOpen, onClose]);

  const attachTarget = (node: HTMLElement | null): void => {
    if (node !== null && !scrolledRef.current) {
      scrolledRef.current = true;
      node.scrollIntoView({ block: 'start' });
    }
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

      {/* Закреплённая шапка: поля границ «с …/по …» (следуют за тапами
          вживую), чип месяца для прыжка и строка дней недели над
          прокруткой; на десктопе TopNav зафиксирован над экраном. */}
      <div className="shrink-0 tablet:mt-[72px]">
        <div className="mx-auto w-full max-w-[560px]">
          {/* Поля границ «с …/по …» следуют за тапами вживую; без выбора
              (#670) — плейсхолдеры, пока диапазон не начат. */}
          <div aria-live="polite" className="grid grid-cols-2 gap-2 px-6 pt-6">
            <span className="flex h-12 items-center rounded-2xl bg-surface-muted px-4 text-base font-medium leading-[18px] text-content">
              {shown !== null ? `с ${formatRangeBound(shown.from, today)}` : 'с …'}
            </span>
            <span className="flex h-12 items-center rounded-2xl bg-surface-muted px-4 text-base font-medium leading-[18px] text-content">
              {shown !== null ? `по ${formatRangeBound(shown.to, today)}` : 'по …'}
            </span>
          </div>

          {/* Чип прыжка — опциональный (monthJump): на фильтре периода
              операций скрыт (решение владельца 2026-09-05). */}
          {monthJump && (
            <div className="px-6 pt-3">
              <MonthJumpChip
                label={`${MONTH_LABELS[draftMonth.month0]} ${draftMonth.year}`}
                onClick={() => setMonthPickerOpen(true)}
              />
            </div>
          )}

          <div className="mt-3 grid grid-cols-7 gap-0.5 px-5 text-center text-base font-medium leading-[18px] text-content-tertiary">
            {WEEKDAY_LABELS.map((weekday) => (
              <div key={weekday} className="flex h-12 items-center justify-center">
                {weekday}
              </div>
            ))}
          </div>
        </div>
      </div>

      <div ref={scrollRef} onScroll={handleScroll} className="min-h-0 flex-1 overflow-y-auto">
        <div className="mx-auto w-full max-w-[560px] pb-[136px]">
          {months.map(({ year, month0 }, index) => {
            const future = calendarMonthIndex({ year, month0 }) > todayIdx;
            return (
              <section
                key={monthKey(year, month0)}
                ref={(node) => {
                  const key = monthKey(year, month0);
                  if (node !== null) {
                    monthRefs.current.set(key, node);
                  } else {
                    monthRefs.current.delete(key);
                  }
                  // Первый кадр — к месяцу конца диапазона; без выбора
                  // (#670) — к текущему месяцу (лента стартует пятью
                  // месяцами раньше).
                  const anchorIso = shown !== null ? shown.to : today;
                  if (year === calendarMonthOf(anchorIso).year && month0 === calendarMonthOf(anchorIso).month0) {
                    attachTarget(node);
                  }
                }}
                // Без content-visibility: якорь дорисовки назад считает
                // высоты по реальному layout (оценка офскрина прыгала).
                className={cn(index === 0 ? undefined : 'mt-8', future && 'opacity-50')}
              >
                <h3 className="px-6 text-xl font-semibold leading-6 text-content">
                  {MONTH_LABELS[month0]}, {year}
                </h3>
                <RangeMonthGrid
                  year={year}
                  month0={month0}
                  draft={draft}
                  today={today}
                  onPick={(day) => setDraft(pickIsoRange(draft, day))}
                />
              </section>
            );
          })}
        </div>
      </div>

      {/* «Выбрать» активна только при выборе (#670): пустой черновик
          подтверждать нечего. «Сбросить» — рядом, когда потребителю нужен
          возврат к состоянию «без периода» (канон пары: secondary слева,
          primary справа, как в WizardBottomBar потоков). */}
      <StickyBottomBar fullWidthContent>
        <div className="flex w-full gap-2">
          {onReset !== undefined && (
            <Button variant="secondary" className="w-full" onClick={onReset}>
              Сбросить
            </Button>
          )}
          <Button
            className="w-full"
            disabled={draft === null}
            onClick={() => {
              if (draft !== null) {
                onConfirm(settleIsoRange(draft));
              }
            }}
          >
            {confirmLabel}
          </Button>
        </div>
      </StickyBottomBar>

      {/* Шит месяца и года — только вместе с чипом прыжка (monthJump):
          самостоятельный WheelPickerSheet, монтируется только в открытом
          состоянии. */}
      {monthJump && (
        <MonthYearPicker
          open={monthPickerOpen}
          onOpenChange={setMonthPickerOpen}
          month={draftMonth.month0}
          year={draftMonth.year}
          max={calendarMonthOf(today)}
          onConfirm={(month0, year) => {
            setMonthPickerOpen(false);
            jumpTo(year, month0);
          }}
        />
      )}
    </div>
  );
}

/** Недельная сетка одного месяца диапазона: серые «пилюли»-подложки
 * недель диапазона и синие граничные ячейки (визуал — 1:1 со страницей
 * периода #477). Будущие дни «сегодня» недоступны. Первый тап задаёт
 * границу, второй завершает диапазон в любую сторону; без начатого
 * выбора (#670) подложек и выделений нет. */
function RangeMonthGrid({
  year,
  month0,
  draft,
  today,
  onPick,
}: {
  readonly year: number;
  readonly month0: number;
  readonly draft: IsoRangeDraft | null;
  readonly today: IsoDate;
  readonly onPick: (day: IsoDate) => void;
}): JSX.Element {
  const days = daysInMonth(year, month0);
  const leadingBlanks = firstWeekdayOfMonth(year, month0);
  // Недели с пустыми колонками до 1-го числа и после последнего — пилюли
  // стелятся отрезками внутри одного ряда.
  const weeks: Array<Array<IsoDate | null>> = [];
  let week: Array<IsoDate | null> = Array.from({ length: leadingBlanks }, () => null);
  for (let day = 1; day <= days; day += 1) {
    week.push(isoDateOf(year, month0, day));
    if (week.length === 7) {
      weeks.push(week);
      week = [];
    }
  }
  if (week.length > 0) {
    weeks.push([...week, ...Array.from({ length: 7 - week.length }, () => null)]);
  }

  const complete =
    draft !== null && draft.end !== null ? { from: draft.start, to: draft.end } : null;

  return (
    <div className="mt-4 flex flex-col gap-0.5 px-4">
      {weeks.map((weekDays, weekIndex) => (
        <RangeWeekRow
          key={weekIndex}
          days={weekDays}
          draft={draft}
          complete={complete}
          today={today}
          onPick={onPick}
        />
      ))}
    </div>
  );
}

/** Недельный ряд: серые отрезки диапазона + ячейки дней одного грида. */
function RangeWeekRow({
  days,
  draft,
  complete,
  today,
  onPick,
}: {
  readonly days: ReadonlyArray<IsoDate | null>;
  readonly draft: IsoRangeDraft | null;
  readonly complete: IsoRange | null;
  readonly today: IsoDate;
  readonly onPick: (day: IsoDate) => void;
}): JSX.Element {
  const inRange = days.map(
    (day) => day !== null && complete !== null && day >= complete.from && day <= complete.to,
  );
  const segments = booleanRunSegments(inRange);

  return (
    <div className="grid grid-cols-7 gap-0.5">
      {segments.map(([from, to]) => (
        <div
          key={`segment-${from}-${to}`}
          aria-hidden
          className="row-start-1 h-full rounded-xl bg-surface-muted"
          style={{ gridColumn: `${from + 1} / ${to + 2}`, gridRow: 1 }}
        />
      ))}
      {days.map((day, column) =>
        day === null ? (
          <div
            key={`blank-${column}`}
            className="row-start-1 aspect-square"
            style={{ gridColumn: column + 1, gridRow: 1 }}
          />
        ) : (
          <button
            key={day}
            type="button"
            disabled={day > today}
            onClick={() => onPick(day)}
            aria-pressed={draft !== null && (day === draft.start || day === draft.end)}
            className={cn(
              'aspect-square w-full cursor-pointer rounded-xl text-center font-sans text-base font-medium leading-[18px] text-content outline-none transition-colors',
              'focus-visible:ring-2 focus-visible:ring-primary',
              'disabled:pointer-events-none disabled:opacity-50',
              draft !== null && (day === draft.start || day === draft.end)
                ? 'bg-primary text-white hover:bg-primary-hover active:bg-primary-active'
                : inRange[column] === true
                  ? 'bg-transparent hover:bg-black/5 active:bg-black/10'
                  : 'bg-transparent hover:bg-surface-muted active:bg-surface-muted-hover',
            )}
            style={{ gridColumn: column + 1, gridRow: 1 }}
          >
            {isoDayOfMonth(day)}
          </button>
        ),
      )}
    </div>
  );
}
