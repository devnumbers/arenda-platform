'use client';

import {
  memo,
  useCallback,
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
  fullscreenSurfaceClass,
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
  CALENDAR_FEED_BACKWARD_SEED,
  CALENDAR_FEED_MONTHS,
  CALENDAR_FEED_PREPEND_CHUNK,
  calendarDayDisabled,
  calendarFeedStart,
  calendarMonthIndex,
  calendarMonthOf,
  calendarMonthOfIndex,
  calendarPastFloor,
  calendarSingleFeedWindow,
  type CalendarMonthRef,
  type IsoDate,
  type IsoRange,
  type IsoRangeDraft,
  isoDateOf,
  isoDayOfMonth,
  listCalendarMonths,
  pickIsoRange,
  RANGE_FEED_BACKWARD_SEED,
  rangeFeedWindow,
  settleIsoRange,
} from '@/shared/lib/calendar';
import { formatRangeBound } from '@/shared/lib/date-format';
import { useBodyScrollLock } from '@/shared/lib/hooks/useBodyScrollLock';
import { useVirtualizer } from '@tanstack/react-virtual';
import {
  createPrependGate,
  flingChunks,
  shouldDeferPrepend,
} from './calendar-feed-gate';
import {
  calendarMonthRowHeights,
  rowOffset,
} from './calendar-month-metrics';
import { MONTH_LABELS, daysInMonth, firstWeekdayOfMonth, WEEKDAY_LABELS } from '@/shared/ui/design/month-grid';

/** SSR-безопасный layout-эффект: запись позиции до пейнта (планы 049/051). */
const useIsomorphicLayoutEffect =
  typeof window === 'undefined' ? useEffect : useLayoutEffect;

/** Хвост прокрутки под последним месяцем — воздух под нижней панелью
 * (бывший pb-[136px] контейнера ленты, план 052). */
const FEED_TAIL_PX = 136;

/** Пауза повтора отложенной приписки (моментум-окно, план 052): за это
 * время хвост затухания флика опускается ниже порога скорости. */
const DEFER_RETRY_MS = 160;

/** Полноэкранный пикер даты (общий компонент дизайн-слоя, вырос из пикера
 * задач #500, Figma 1539-78660): назад, чип «Август 2026 ⌄» (шит
 * MonthYearPicker) и шапка дней недели закреплены над прокруткой — лента
 * месяцев скроллится под ними (решение владельца 2026-09-04). Лента
 * стартует на стартовый запас (seed 12, план 051) раньше якоря —
 * прокрутка вверх работает сразу, без «качания» (решение владельца
 * 13.10, #1394: на верхнем упоре scroll-событие не возникает, дорисовка
 * по порогу не запускается) и вне зоны приписки с первого пикселя;
 * оффскрин-блоки не участвуют в layout (content-visibility) — прокрутка
 * остаётся плавной на любой глубине. Открывается сеткой на
 * месяце якоря — значения, а без него «сегодня», прижатое в
 * [minDate, maxDate] (решение владельца #802 23.09: продление с далёким
 * минимумом открывается на первом доступном месяце). Черновик предвыбран
 * сразу — «сегодня» либо граница [minDate, maxDate] (продление #802
 * прижимает к первому доступному месяцу), поэтому «Выбрать» активна без
 * действий (подсказка владельца к #500); дни раньше сегодня недоступны —
 * задним числом даты не выбираются (контракт #498), кроме текущего
 * значения-якоря в прошлом при правке (#502). Рендерится только в
 * открытом состоянии — состояние ленты и черновик живут, пока пикер
 * смонтирован.
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
 * Открытое прошлое (проп allowPast, #805): прошедшие дни до minDate
 * тапабельны, лента стартует от месяца minDate — дата завершения аренды
 * бывает задним числом (ADR 0053 §3: start ≤ дата ≤ сегодня). Без пропа
 * прошлое закрыто сегодняшним (контракт #498); проп не трогает якорь
 * значения-в-прошлом правки (#502) и границы maxDate.
 *
 * Прошлое без пола (allowPast без minDate, #1393 — операции задним
 * числом): черты нет, прошедшие дни открыты без ограничения; лента
 * растёт назад по доскроллу — порции по 6 месяцев, позиция прокрутки не
 * дёргается (тот же приём, что в пикере периода); шит месяца без нижнего
 * зажима — годы уходят в прошлое по колёснику.
 *
 * Будущее с жёстким краем (maxDate задан, решение владельца 13.10,
 * #1394): дни позже недоступны (#534), лента заканчивается месяцем maxDate
 * плюс парой приглушённых месяцев контекста — дальше не дорисовывается, и
 * шит месяца зажат по maxDate (1:1 с пикером периода). У открытого
 * будущего (maxDate не задан — начало аренды, плановые платежи) края нет:
 * лента растёт вперёд годом, как раньше.
 *
 * Максимальная дата (проп maxDate, #534): дни позже недоступны — дата
 * завершения аренды не бывает будущей (ADR 0053 §3, потребитель передаёт
 * «сегодня» собственника). Симметрично минимуму: пустой черновик
 * прижимается к границе, если «сегодня» за ней.
 *
 * Планшет (561–1023): TopNav и белый шит футера тянутся во всю ширину —
 * кнопка «Выбрать» тоже (StickyBottomBar, универсальное правило планшета:
 * нижние панели во всю ширину, на ПК ≥1024 — колонка 560; решение
 * владельца 28.09 после аудита #877, канон DESIGN.md §5). */

/** Порог дорисовки вперёд: до нижнего края осталось ~5 экранов —
 * запас на флик (план 042, feel-check владельца 16.10). */
const APPEND_THRESHOLD_PX = 1500;

/** Число порций prepend под текущую скорость прокрутки (план 042):
 * одна порция на медленном скролле; под фликом — столько, сколько
 * моментум съест за ~600мс после события, с кэпом FLING_CHUNKS_CAP
 * (план 051: монструозные коммиты 48 месяцев душили телефон). Сама
 * функция живёт в calendar-feed-gate — модуль тайминга ленты. */
const FEED_CHUNK_PX = 2800;

function monthKey(year: number, month0: number): string {
  return `${year}-${month0}`;
}

/** Блок месяца ленты (мемо, план 050): приписка перерисовывает только
 * новые блоки — существующие выпадают из реконсиляции. С плана 052 блок
 * живёт внутри виртуальной строки: межстрочный отступ и позицию даёт
 * обёртка виртуализатора, ref-регистрация не нужна (позиции — точная
 * формула calendar-month-metrics + measureElement самокоррекции). */
const CalendarMonthBlock = memo(function CalendarMonthBlock({
  year,
  month0,
  beyondMaxDate,
  today,
  draft,
  value,
  minDate,
  maxDate,
  allowPast,
  onSelect,
}: {
  readonly year: number;
  readonly month0: number;
  readonly beyondMaxDate: boolean;
  readonly today: IsoDate;
  readonly draft: IsoDate | null;
  readonly value: IsoDate | null;
  readonly minDate: IsoDate | undefined;
  readonly maxDate: IsoDate | undefined;
  readonly allowPast: boolean;
  readonly onSelect: (iso: IsoDate, selected: boolean) => void;
}): JSX.Element {
  const days = daysInMonth(year, month0);
  const leadingBlanks = firstWeekdayOfMonth(year, month0);
  return (
    <section className={cn(beyondMaxDate && 'opacity-50')}>
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
              // Прошлые дни закрыты полом (calendarDayDisabled):
              // сегодня по умолчанию — задним числом даты не
              // создаются (#498), minDate при allowPast — дата
              // завершения бывает задним числом (#805); текущее
              // значение-якорь в прошлом остаётся тапабельным
              // (правка #502), повторный тап снимает дату. Дни
              // раньше minDate недоступны без исключений (minDate —
              // первый доступный: окончание аренды строго позже
              // начала, ADR 0053); дни позже maxDate — тоже (дата
              // завершения не бывает будущей, #534).
              disabled={calendarDayDisabled({
                iso,
                today,
                value,
                minDate,
                maxDate,
                allowPast,
              })}
              // Повторный тап по выбранному дню снимает выбор
              // (решение владельца 2026-09-03); с required дата
              // обязательна — тап её держит.
              onClick={() => onSelect(iso, selected)}
            >
              {day + 1}
            </CalendarButton>
          );
        })}
      </div>
    </section>
  );
});

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
  /** Прошедшие дни открыты — дата бывает задним числом. С minDate открыты
   * до minDate (мастер завершения аренды, ADR 0053 §3: start ≤ дата ≤
   * сегодня, #805); без minDate — без ограничения (операции задним числом,
   * #1393): лента растёт назад по доскроллу, шит месяца без нижнего
   * зажима. Без пропа прошлое закрыто сегодняшним (контракт #498); проп
   * не трогает якорь значения-в-прошлом правки (#502) и границы maxDate. */
  readonly allowPast?: boolean;
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
  allowPast = false,
  onClose,
  onConfirm,
}: CalendarDatePickerProps): JSX.Element {
  // Поверхность fixed без модальной обёртки: страница под пикером
  // скроллиться не должна (жест у края ленты раньше уезжал в неё).
  useBodyScrollLock();
  // Лента (решение владельца 13.10, #1394): старт — на порцию дорисовки
  // раньше семантического старта calendarFeedStart — над якорем всегда
  // есть содержимое, прокрутка вверх работает сразу (на верхнем упоре
  // scroll-событие не возникает, дорисовка по порогу не запускалась —
  // «сначала качни вниз»); якорь скроллится на место в layout-эффекте до
  // первой отрисовки. Вперёд при закрытом будущем (maxDate) — жёсткий
  // край: месяц maxDate плюс пара приглушённых месяцев контекста, дальше
  // лента не растёт (1:1 с пикером периода); у открытого будущего края
  // нет — лента растёт годом.
  const initialAnchor = value ?? initialDraft(today, minDate, maxDate);
  const pastFloor = calendarPastFloor(today, minDate, allowPast);
  const endlessPast = pastFloor === null;
  const feedStart = calendarFeedStart(
    today,
    value,
    pastFloor !== null && pastFloor !== today ? calendarMonthOf(pastFloor) : undefined,
  );
  const feedWindow = calendarSingleFeedWindow({
    feedStart,
    anchorMonth: calendarMonthOf(initialAnchor),
    maxDate,
  });
  const [firstMonth, setFirstMonth] = useState<CalendarMonthRef>(feedWindow.first);
  const [monthsCount, setMonthsCount] = useState(feedWindow.count);
  // Край ленты стабилен всю жизнь смонтированного пикера (пропсы не
  // меняются — компонент рендерится только в открытом состоянии).
  const forwardEdge = feedWindow.forwardEdge;
  const [draft, setDraft] = useState<IsoDate | null>(
    value ?? initialDraft(today, minDate, maxDate),
  );
  const [monthPickerOpen, setMonthPickerOpen] = useState(false);
  const startIndex = calendarMonthIndex(firstMonth);
  const anchorIndex =
    calendarMonthIndex(calendarMonthOf(initialAnchor)) - startIndex;
  const months = listCalendarMonths(firstMonth, monthsCount);
  const scrollRef = useRef<HTMLDivElement | null>(null);
  // Прыжок за окно ленты (чип «Месяц Год»): абсолютный индекс месяца —
  // окно переносится к цели, скролл после рендера нового окна.
  const pendingScrollIndex = useRef<number | null>(null);
  // Гейт приписки (план 050): одна порция в полёте — скролл-события
  // одного кадра не суммируются в один DOM-коммит.
  const [prependGate] = useState(createPrependGate);
  // Компенсация prepend (планы 051→052): дельта = сумма точных высот
  // новых строк (calendar-month-metrics), пишется в scrollTop до пейнта.
  const pendingPrependDelta = useRef<number | null>(null);
  // Ширина колонки — вход точных высот строк. Замеряется в layout-эффекте
  // первого коммита (до пейнта) и держится ResizeObserver'ом; до замера
  // строки не монтируются — первый закрашенный кадр уже с точными
  // размерами и якорем (план 052: дрейф оценок открытия устранён).
  const contentRef = useRef<HTMLDivElement | null>(null);
  const [columnWidth, setColumnWidth] = useState<number | null>(null);
  useIsomorphicLayoutEffect(() => {
    if (columnWidth !== null) {
      return;
    }
    const node = contentRef.current;
    if (node !== null && node.clientWidth > 0) {
      setColumnWidth(node.clientWidth);
    }
  }, [columnWidth]);
  useEffect(() => {
    const node = contentRef.current;
    if (node === null || typeof ResizeObserver === 'undefined') {
      return;
    }
    const observer = new ResizeObserver((entries) => {
      const width = entries[0]?.contentRect.width;
      if (width !== undefined && width > 0) {
        setColumnWidth(width);
      }
    });
    observer.observe(node);
    return () => observer.disconnect();
  }, []);
  // Выбор дня — стабильный колбэк (проп мемо-блока): повторный тап
  // снимает выбор, required держит дату.
  const selectDraft = useCallback(
    (iso: IsoDate, selected: boolean) => {
      setDraft(selected && !required ? null : iso);
    },
    [required],
  );

  // Чип месяца показывает месяц черновика; у пустого — текущий месяц.
  const draftMonth = calendarMonthOf(draft ?? today);

  // Виртуальная лента (план 052): в DOM только видимое окно + overscan;
  // высоты строк — точная формула по месяцам (месяц известен заранее),
  // measureElement только дотягивает субпиксель.
  const rowHeights = calendarMonthRowHeights(months, columnWidth);
  // Инстанс рождается с якорным offset (initialOffset — ленивый, читается
  // при первой подписке): окно первого кадра считает себя по формуле с
  // шириной из токена колонки, не дожидаясь первого read-back'а — гонка
  // WebKit «контент на месте, окно рендера от нуля» (репро 16.10)
  // исключена; точную позицию ставит якорный эффект ниже.
  const [bornOffset] = useState(() => {
    if (typeof window === 'undefined') {
      return 0;
    }
    const columnToken = parseFloat(
      getComputedStyle(document.documentElement).getPropertyValue('--container-column'),
    );
    const column = Math.min(Number.isFinite(columnToken) ? columnToken : 560, window.innerWidth);
    return rowOffset(
      calendarMonthRowHeights(months.slice(0, Math.max(anchorIndex, 0)), column),
      Math.max(anchorIndex, 0),
    );
  });
  const virtualizer = useVirtualizer({
    count: months.length,
    getScrollElement: () => scrollRef.current,
    estimateSize: (index) => rowHeights[index] ?? 0,
    getItemKey: (index) => {
      const month = months[index];
      return month === undefined ? String(index) : monthKey(month.year, month.month0);
    },
    overscan: 3,
    initialOffset: bornOffset,
  });

  // Открытие — сразу на месяц якоря (значение или ближайший доступный
  // день): scrollTop = сумма точных высот до якоря, месяц ровно под
  // верхней кромкой на любой ширине. Layout-эффект — до отрисовки, и
  // строки к этому моменту уже смонтированы с точными размерами (замер
  // колонки выше). Под якорем должно хватить контента на вьюпорт, иначе
  // запись упрётся в clamp дна и месяц сядет ниже кромки (репро 16.10:
  // безпольный пикер открывался сентябрём — якорь-октябрь был последней
  // строкой окна): у открытого будущего окно доращивается вперёд и
  // эффект повторяется в том же sync-ререндере до пейнта; при жёстком
  // крае (maxDate) расти некуда — садимся максимально низко, как раньше.
  // Один раз: дальше старт ленты уплывает легитимно.
  const anchorScrolledRef = useRef(false);
  useIsomorphicLayoutEffect(() => {
    if (anchorScrolledRef.current || columnWidth === null || anchorIndex <= 0) {
      return;
    }
    const list = scrollRef.current;
    if (list === null) {
      return;
    }
    const target = rowOffset(rowHeights, anchorIndex);
    if (target > list.scrollHeight - list.clientHeight) {
      if (forwardEdge === null) {
        // Порядка вьюпорта месяцев ниже якоря + запас под append.
        setMonthsCount((count) => count + Math.ceil(list.clientHeight / 240) + 2);
        return;
      }
    }
    anchorScrolledRef.current = true;
    // Запись только через API виртуализатора: scrollToOffset обновляет
    // его внутренний scrollOffset синхронно — сырое scrollTop = value
    // гонку инициализации слушателя на WebKit проигрывало, и
    // виртуализатор держал окно от нуля (репро 16.10: контент на месте,
    // окно рендера — строки 0..4).
    virtualizer.scrollToOffset(target);
  }, [anchorIndex, columnWidth, forwardEdge, rowHeights, virtualizer]);

  // Дорисовка прыжка за окно — скролл к цели после рендера нового окна,
  // до пейнта (запись позиции в коммите окна — канон 049/051/052).
  useIsomorphicLayoutEffect(() => {
    // Приписка доехала до DOM — гейт открывает следующий запрос (план 050).
    prependGate.settled();
    const absIndex = pendingScrollIndex.current;
    if (absIndex === null) {
      return;
    }
    pendingScrollIndex.current = null;
    if (columnWidth === null) {
      return;
    }
    const targetIndex = absIndex - startIndex;
    // Страховка WebKit: read-back записи доезжает с лагом — проверяем окно
    // и при промахе переписываем (до ~0.5с). setTimeout, не rAF:
    // headless-браузер e2e не обязан тикать кадры (канон useBodyScrollLock).
    let guardFrames = 0;
    let guardTimer = 0;
    const guard = (): void => {
      guardFrames += 1;
      const mounted = virtualizer.getVirtualItems().some((v) => v.index === targetIndex);
      if (!mounted && guardFrames < 30) {
        if (guardFrames === 3) {
          // Сброс кэша измерений: застывший расчёт диапазона не нотифицирует
          // ререндер (webkit-гонка первого read-back'а).
          virtualizer.measure();
        }
        virtualizer.scrollToIndex(targetIndex, { align: 'start' });
        guardTimer = window.setTimeout(guard, 16);
      }
    };
    virtualizer.scrollToIndex(targetIndex, { align: 'start' });
    guardTimer = window.setTimeout(guard, 16);
    const cleanupGuard = (): void => window.clearTimeout(guardTimer);
    return cleanupGuard;
  }, [monthsCount, firstMonth, prependGate, columnWidth, rowHeights, startIndex, virtualizer]);

  // Компенсация prepend (планы 051→052): до пейнта возвращаем позицию,
  // сдвинутую вставкой порции над вьюпортом. Дельта — чистая математика
  // поверх точных высот (без DOM-измерений, план 052); пишем в текущий
  // scrollTop — моментум между событием и коммитом сохраняется.
  useIsomorphicLayoutEffect(() => {
    const delta = pendingPrependDelta.current;
    pendingPrependDelta.current = null;
    const list = scrollRef.current;
    if (delta === null || list === null) {
      return;
    }
    virtualizer.scrollToOffset(list.scrollTop + delta);
  }, [monthsCount, firstMonth, virtualizer]);

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
  // месяцы есть всегда, пока будущее открыто; при закрытом (maxDate, жёсткий
  // край #1394) лента заканчивается парой приглушённых месяцев за краем и
  // не растёт. В безпольном режиме (#1393) дотягивание к верху дорисовывает
  // порцию месяцев назад. Лента виртуальна (план 052): в DOM только
  // видимое окно + overscan, глубина не стоит ни размеров, ни коммитов.
  // Сдвиг позиции при prepend — компенсация до пейнта (дельта точных
  // высот), на ленте [overflow-anchor:none] — одна компенсация на всех
  // движках (планы 051→052). Моментум-окно (план 052, решение владельца):
  // на высокой скорости DOM не трогается вовсе — приписка откладывается
  // и доезжает в тишине (повтор по таймеру, DEFER_RETRY_MS); жест несут
  // точные размеры окна и seed. Порог 3000 держит триггер далеко от
  // стены; кэп flingChunks 2 (план 051).
  const lastScrollRef = useRef<{ st: number; t: number } | null>(null);
  const deferredPrependTimer = useRef<number | null>(null);
  const requestPrepend = (
    previous: { st: number; t: number } | null,
    now: number,
    st: number,
  ): void => {
    const chunks = flingChunks(previous, now, st, FEED_CHUNK_PX);
    // Склейка по кадру (план 050): пока предыдущая приписка в полёте,
    // запросы склеиваются с ней; settled() зовёт эффект рендера приписки.
    const portion = prependGate.request(chunks);
    if (portion === null) {
      return;
    }
    const newCount = CALENDAR_FEED_PREPEND_CHUNK * portion;
    const newFirst = calendarMonthOfIndex(startIndex - newCount);
    // Дельта компенсации — прирост ленты над вьюпортом: смещение первой
    // старой строки в новом окне (последняя новая строка уже несёт свой
    // межстрочный отступ). План 052, без DOM-измерений.
    const newHeights = calendarMonthRowHeights(
      listCalendarMonths(newFirst, monthsCount + newCount),
      columnWidth,
    );
    pendingPrependDelta.current = rowOffset(newHeights, newCount);
    setFirstMonth(newFirst);
    setMonthsCount((count) => count + newCount);
  };
  const scheduleDeferredPrepend = (): void => {
    if (deferredPrependTimer.current !== null) {
      return;
    }
    deferredPrependTimer.current = window.setTimeout(() => {
      deferredPrependTimer.current = null;
      const list = scrollRef.current;
      if (list === null || list.scrollTop > PREPEND_THRESHOLD_PX) {
        return;
      }
      const now = performance.now();
      const previous = lastScrollRef.current;
      lastScrollRef.current = { st: list.scrollTop, t: now };
      if (shouldDeferPrepend(previous, now, list.scrollTop)) {
        scheduleDeferredPrepend();
        return;
      }
      requestPrepend(previous, now, list.scrollTop);
    }, DEFER_RETRY_MS);
  };
  useEffect(
    () => () => {
      if (deferredPrependTimer.current !== null) {
        window.clearTimeout(deferredPrependTimer.current);
      }
    },
    [],
  );
  const handleScroll = (): void => {
    const list = scrollRef.current;
    if (list === null) return;
    const now = performance.now();
    const previous = lastScrollRef.current;
    lastScrollRef.current = { st: list.scrollTop, t: now };
    if (endlessPast && list.scrollTop <= PREPEND_THRESHOLD_PX) {
      if (shouldDeferPrepend(previous, now, list.scrollTop)) {
        scheduleDeferredPrepend();
        return;
      }
      requestPrepend(previous, now, list.scrollTop);
      return;
    }
    if (
      forwardEdge === null
      && list.scrollTop + list.clientHeight >= list.scrollHeight - APPEND_THRESHOLD_PX
    ) {
      setMonthsCount((count) => count + CALENDAR_FEED_MONTHS);
    }
  };

  /** Прыжок по чипу: месяц окна — скроллом по точным высотам; за окном —
   * перенос окна к цели целиком (виртуализатор рендерит только видимое,
   * расстояние прыжка не стоит ни коммитов, ни кадров — план 052) и
   * скролл после рендера. Вперёд лента растёт годом; при закрытом будущем
   * прыжок ограничен краем (колесо годов и так не предложит месяц за
   * maxDate). */
  const jumpTo = (year: number, month0: number): void => {
    const targetIdx = calendarMonthIndex({ year, month0 });
    const relative = targetIdx - startIndex;
    if (relative >= 0 && relative < monthsCount) {
      virtualizer.scrollToIndex(relative, { align: 'start' });
      return;
    }
    pendingScrollIndex.current = targetIdx;
    if (targetIdx < startIndex) {
      setFirstMonth(calendarMonthOfIndex(targetIdx - CALENDAR_FEED_BACKWARD_SEED));
      setMonthsCount((count) => count + (startIndex - (targetIdx - CALENDAR_FEED_BACKWARD_SEED)));
      return;
    }
    setMonthsCount(
      forwardEdge !== null
        ? calendarMonthIndex(forwardEdge) - startIndex + 1
        : relative + CALENDAR_FEED_MONTHS,
    );
  };

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label={title}
      className={cn(fullscreenSurfaceClass, 'font-sans')}
    >
      <TopNav
        overlay
        leading={<IconButton icon={<ArrowLeft />} label="Назад" onClick={onClose} />}
      >
        <TopNavTitle title={title} />
      </TopNav>

      {/* Закреплённая шапка: чип месяца и строка дней недели над прокруткой
          (требование владельца к бесконечному календарю); на десктопе TopNav
          зафиксирован над экраном — шапка встаёт под ним, лента скроллится
          между шапкой и нижней панелью. */}
      <div className="relative shrink-0 tablet:mt-[72px]">
        <div className="mx-auto w-full max-w-column">
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

      {/* Виртуальная лента (план 052): спейсер — колонка контента, строки
          позиционированы абсолютно, в DOM только видимое окно + overscan.
          Строки не монтируются до замера колонки — первый закрашенный кадр
          уже с точными размерами и якорем открытия. Позицию при приписке
          держит компенсация до пейнта; [overflow-anchor:none] выключает
          нативный scroll anchoring: в WebKit его не было до Safari 27, с
          ручной компенсацией нативная удвоила бы сдвиг на Chromium. */}
      <div
        ref={scrollRef}
        onScroll={handleScroll}
        className="relative min-h-0 flex-1 overflow-y-auto overscroll-contain [scrollbar-width:none] [overflow-anchor:none]"
      >
        <div
          ref={contentRef}
          className="relative mx-auto w-full max-w-column"
          style={{ height: columnWidth === null ? 0 : virtualizer.getTotalSize() + FEED_TAIL_PX }}
        >
          {columnWidth !== null
            && virtualizer.getVirtualItems().map((row) => {
              const month = months[row.index];
              if (month === undefined) {
                return null;
              }
              // Приглушённое будущее за месяцем maxDate (жёсткий край
              // #1394): контекст после черты виден, но выбирать там нечего
              // — 1:1 с пикером периода.
              const beyondMaxDate =
                forwardEdge !== null
                && maxDate !== undefined
                && calendarMonthIndex(month) > calendarMonthIndex(calendarMonthOf(maxDate));
              return (
                <div
                  key={row.key}
                  data-index={row.index}
                  ref={virtualizer.measureElement}
                  className="absolute left-0 w-full"
                  style={{
                    transform: `translateY(${row.start}px)`,
                    paddingBottom: row.index === months.length - 1 ? 0 : 32,
                  }}
                >
                  <CalendarMonthBlock
                    year={month.year}
                    month0={month.month0}
                    beyondMaxDate={beyondMaxDate}
                    today={today}
                    draft={draft}
                    value={value}
                    minDate={minDate}
                    maxDate={maxDate}
                    allowPast={allowPast}
                    onSelect={selectDraft}
                  />
                </div>
              );
            })}
        </div>
      </div>

      {/* Кнопка скрыта, пока нечего подтвердить (обязательная дата без
          черновика) — решение владельца 2026-09-05: скрытие вместо
          погашенной кнопки, у пикерных «Выбрать» так же. Без required
          активна всегда: с пустым черновиком подтверждает «без даты».
          Футер тянется вместе с шитом в планшетном диапазоне 561–1023
          (универсальное правило планшета, решение владельца 28.09, #877:
          на ПК ≥1024 — колонка 560, канон DESIGN.md §5). */}
      {!(required === true && draft === null) && (
        <StickyBottomBar>
          <Button className="w-full" onClick={() => onConfirm(draft)}>
            {confirmLabel}
          </Button>
        </StickyBottomBar>
      )}

      {/* Шит месяца и года — самостоятельный WheelPickerSheet (общий
          компонент), монтируется только в открытом состоянии; в безпольном
          режиме (#1393) нижнего зажима нет — годы уходят в прошлое по
          колёснику (onNearStart), месяцы не обрезаются. Будущее закрыто
          зажимом по maxDate (жёсткий край #1394): дальше черты колесо не
          предложит — 1:1 с пикером периода. */}
      <MonthYearPicker
        open={monthPickerOpen}
        onOpenChange={setMonthPickerOpen}
        month={draftMonth.month0}
        year={draftMonth.year}
        min={endlessPast ? undefined : feedStart}
        max={maxDate !== undefined ? calendarMonthOf(maxDate) : undefined}
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
      className="inline-flex h-11 cursor-pointer items-center gap-2 rounded-pill bg-surface-muted px-5 text-base font-medium text-content outline-none transition-colors hover:bg-surface-muted-hover active:bg-surface-muted-active focus-visible:ring-2 focus-visible:ring-primary"
    >
      {label}
      <SmallArrowDown className="h-6 w-6 text-content-secondary" aria-hidden />
    </button>
  );
}

/** Порог дорисовки назад безпольной ленты одиночного пикера: до верха
 * контейнера осталось меньше этого (#1393). */
const PREPEND_THRESHOLD_PX = 3000;
/** Порог дорисовки назад ленты пикера периода: ~5 экранов прокрутки.
 * Порция обязана отрисоваться задолго до стены: прежние 32px (потом 800)
 * срабатывали вплотную к краю — под жёстким фликом (~100px/мс) лента
 * жила циклом «стена → компенсация прыжком на порцию» (feel-check
 * владельца 16.10, замер 16.10: паузы на scrollTop 0 между каждой
 * дорисовкой). */
const RANGE_PREPEND_THRESHOLD_PX = 3000;
/** Сколько месяцев дорисовывается назад в ленте периода за одно
 * дотягивание к верху (решение владельца 13.10: порция 6 остаётся). */
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
 * (универсальное правило планшета, решение владельца 28.09). Рендерится
 * только в открытом состоянии — лента и черновик живут, пока пикер
 * смонтирован. */
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
  // Поверхность fixed без модальной обёртки: страница под пикером
  // скроллиться не должна (жест у нижнего края ленты раньше уезжал в неё).
  useBodyScrollLock();
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

  const scrollRef = useRef<HTMLDivElement | null>(null);
  // Темп прокрутки для скоростно-зависимого prepend (flingChunks).
  const lastRangeScrollRef = useRef<{ st: number; t: number } | null>(null);
  // Гейт приписки (план 050, дотянут до range планом 051): одна порция в
  // полёте — скролл-события кадра не складываются в один коммит.
  const [rangePrependGate] = useState(createPrependGate);
  // Компенсация prepend (планы 051→052): дельта = сумма точных высот новых
  // строк, запись scrollTop до пейнта.
  const rangePrependDelta = useRef<number | null>(null);
  // Месяц прыжка чипа за окном ленты — скролл после рендера нового окна.
  const pendingJumpIndex = useRef<number | null>(null);
  // Первый рендер — сразу к месяцу конца применённого диапазона.
  const scrolledRef = useRef(false);
  // Ширина колонки — вход точных высот строк (план 052): замер в первом
  // layout-проходе, строки не монтируются до замера (открытие без дрейфа
  // оценок); дальше — ResizeObserver.
  const contentRef = useRef<HTMLDivElement | null>(null);
  const [columnWidth, setColumnWidth] = useState<number | null>(null);
  useIsomorphicLayoutEffect(() => {
    if (columnWidth !== null) {
      return;
    }
    const node = contentRef.current;
    if (node !== null && node.clientWidth > 0) {
      setColumnWidth(node.clientWidth);
    }
  }, [columnWidth]);
  useEffect(() => {
    const node = contentRef.current;
    if (node === null || typeof ResizeObserver === 'undefined') {
      return;
    }
    const observer = new ResizeObserver((entries) => {
      const width = entries[0]?.contentRect.width;
      if (width !== undefined && width > 0) {
        setColumnWidth(width);
      }
    });
    observer.observe(node);
    return () => observer.disconnect();
  }, []);

  // Виртуальная лента (план 052): в DOM только видимое окно + overscan.
  // initialOffset — как у одиночного пикера: окно первого кадра рождается
  // у якоря (месяц конца диапазона, без выбора #670 — текущий), гонка
  // первого read-back'а исключена.
  const rangeRowHeights = calendarMonthRowHeights(months, columnWidth);
  const [bornOffset] = useState(() => {
    if (typeof window === 'undefined') {
      return 0;
    }
    const columnToken = parseFloat(
      getComputedStyle(document.documentElement).getPropertyValue('--container-column'),
    );
    const column = Math.min(Number.isFinite(columnToken) ? columnToken : 560, window.innerWidth);
    const anchorIdx =
      calendarMonthIndex(calendarMonthOf(value !== null ? value.to : today))
      - calendarMonthIndex(feedWindow.first);
    return rowOffset(
      calendarMonthRowHeights(months.slice(0, Math.max(anchorIdx, 0)), column),
      Math.max(anchorIdx, 0),
    );
  });
  const virtualizer = useVirtualizer({
    count: months.length,
    getScrollElement: () => scrollRef.current,
    estimateSize: (index) => rangeRowHeights[index] ?? 0,
    getItemKey: (index) => {
      const month = months[index];
      return month === undefined ? String(index) : monthKey(month.year, month.month0);
    },
    overscan: 3,
    initialOffset: bornOffset,
  });

  // Первый рендер — сразу к месяцу конца применённого диапазона (без
  // выбора #670 — к текущему): scrollTop = сумма точных высот, месяц ровно
  // под верхней кромкой (план 052 — дрейф оценок открытия устранён).
  const rangeAnchorIdx =
    calendarMonthIndex(calendarMonthOf(shown !== null ? shown.to : today))
    - calendarMonthIndex(firstMonth);
  useIsomorphicLayoutEffect(() => {
    if (scrolledRef.current || columnWidth === null) {
      return;
    }
    const list = scrollRef.current;
    if (list === null) {
      return;
    }
    scrolledRef.current = true;
    virtualizer.scrollToOffset(rowOffset(rangeRowHeights, rangeAnchorIdx));
  }, [columnWidth, rangeAnchorIdx, rangeRowHeights, virtualizer]);

  useIsomorphicLayoutEffect(() => {
    const jump = pendingJumpIndex.current;
    if (jump === null) {
      return;
    }
    pendingJumpIndex.current = null;
    if (columnWidth === null) {
      return;
    }
    const rangeTargetIndex = jump - calendarMonthIndex(firstMonth);
    let rangeGuardFrames = 0;
    let rangeGuardTimer = 0;
    const rangeGuard = (): void => {
      rangeGuardFrames += 1;
      const mounted = virtualizer.getVirtualItems().some((v) => v.index === rangeTargetIndex);
      if (!mounted && rangeGuardFrames < 30) {
        if (rangeGuardFrames === 3) {
          virtualizer.measure();
        }
        virtualizer.scrollToIndex(rangeTargetIndex, { align: 'start' });
        rangeGuardTimer = window.setTimeout(rangeGuard, 16);
      }
    };
    virtualizer.scrollToIndex(rangeTargetIndex, { align: 'start' });
    rangeGuardTimer = window.setTimeout(rangeGuard, 16);
    return () => window.clearTimeout(rangeGuardTimer);
  }, [columnWidth, firstMonth, rangeRowHeights, virtualizer]);

  // Приписка доехала до DOM — гейт открывает следующий запрос (план 050).
  useEffect(() => {
    rangePrependGate.settled();
  }, [firstMonth, rangePrependGate]);

  // Компенсация prepend (планы 051→052): до пейнта возвращаем позицию,
  // сдвинутую вставкой порции, — дельта точных высот новых строк.
  useIsomorphicLayoutEffect(() => {
    const delta = rangePrependDelta.current;
    rangePrependDelta.current = null;
    const list = scrollRef.current;
    if (delta === null || list === null) {
      return;
    }
    virtualizer.scrollToOffset(list.scrollTop + delta);
  }, [firstMonth, virtualizer]);

  // Отложенная приписка моментум-окна (план 052) — повтор в тишине.
  const deferredPrependTimer = useRef<number | null>(null);

  const requestPrepend = (
    previous: { st: number; t: number } | null,
    now: number,
    st: number,
  ): void => {
    const chunks = flingChunks(previous, now, st, FEED_CHUNK_PX);
    const portion = rangePrependGate.request(chunks);
    if (portion === null) {
      return;
    }
    const newCount = RANGE_PREPEND_CHUNK * portion;
    const newFirst = calendarMonthOfIndex(calendarMonthIndex(firstMonth) - newCount);
    // Дельта — смещение первой старой строки в новом окне (последняя
    // новая строка уже несёт свой межстрочный отступ), план 052.
    rangePrependDelta.current = rowOffset(
      calendarMonthRowHeights(
        listCalendarMonths(
          newFirst,
          calendarMonthIndex(feedWindow.last) - calendarMonthIndex(newFirst) + 1,
        ),
        columnWidth,
      ),
      newCount,
    );
    setFirstMonth(newFirst);
  };

  const scheduleDeferredPrepend = (): void => {
    if (deferredPrependTimer.current !== null) {
      return;
    }
    deferredPrependTimer.current = window.setTimeout(() => {
      deferredPrependTimer.current = null;
      const container = scrollRef.current;
      if (container === null || container.scrollTop > RANGE_PREPEND_THRESHOLD_PX) {
        return;
      }
      const now = performance.now();
      const previous = lastRangeScrollRef.current;
      lastRangeScrollRef.current = { st: container.scrollTop, t: now };
      if (shouldDeferPrepend(previous, now, container.scrollTop)) {
        scheduleDeferredPrepend();
        return;
      }
      requestPrepend(previous, now, container.scrollTop);
    }, DEFER_RETRY_MS);
  };
  useEffect(
    () => () => {
      if (deferredPrependTimer.current !== null) {
        window.clearTimeout(deferredPrependTimer.current);
      }
    },
    [],
  );

  const handleScroll = (): void => {
    const container = scrollRef.current;
    if (container === null) {
      return;
    }
    const now = performance.now();
    const previous = lastRangeScrollRef.current;
    lastRangeScrollRef.current = { st: container.scrollTop, t: now };
    if (container.scrollTop > RANGE_PREPEND_THRESHOLD_PX) {
      return;
    }
    // Моментум-окно (план 052, решение владельца): на высокой скорости DOM
    // не трогается — приписка откладывается и доезжает в тишине.
    if (shouldDeferPrepend(previous, now, container.scrollTop)) {
      scheduleDeferredPrepend();
      return;
    }
    requestPrepend(previous, now, container.scrollTop);
  };

  /** Прыжок по чипу: месяц окна — скроллом по точным высотам; глубже окна
   * — перенос окна к цели целиком (виртуализатор, план 052) и скролл
   * после рендера. */
  const jumpTo = (year: number, month0: number): void => {
    const targetIdx = calendarMonthIndex({ year, month0 });
    if (targetIdx >= calendarMonthIndex(firstMonth)) {
      virtualizer.scrollToIndex(targetIdx - calendarMonthIndex(firstMonth), {
        align: 'start',
      });
      return;
    }
    pendingJumpIndex.current = targetIdx;
    setFirstMonth(calendarMonthOfIndex(targetIdx - RANGE_FEED_BACKWARD_SEED));
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

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label={title}
      className={cn(fullscreenSurfaceClass, 'font-sans')}
    >
      <TopNav
        overlay
        leading={<IconButton icon={<ArrowLeft />} label="Назад" onClick={onClose} />}
      >
        <TopNavTitle title={title} />
      </TopNav>

      {/* Закреплённая шапка: поля границ «с …/по …» (следуют за тапами
          вживую), чип месяца для прыжка и строка дней недели над
          прокруткой; на десктопе TopNav зафиксирован над экраном. */}
      <div className="relative shrink-0 tablet:mt-[72px]">
        <div className="mx-auto w-full max-w-column">
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

      {/* Виртуальная лента (план 052): спейсер — колонка контента, строки
          позиционированы абсолютно, в DOM только видимое окно + overscan;
          строки не монтируются до замера колонки — первый кадр уже с
          точным якорем. Позицию при приписке держит компенсация до
          пейнта; [overflow-anchor:none] держит одну компенсацию (нашу) на
          всех движках — нативного scroll anchoring в WebKit нет до
          Safari 27. */}
      <div
        ref={scrollRef}
        onScroll={handleScroll}
        className="relative min-h-0 flex-1 overflow-y-auto overscroll-contain [scrollbar-width:none] [overflow-anchor:none]"
      >
        <div
          ref={contentRef}
          className="relative mx-auto w-full max-w-column"
          style={{ height: columnWidth === null ? 0 : virtualizer.getTotalSize() + FEED_TAIL_PX }}
        >
          {columnWidth !== null
            && virtualizer.getVirtualItems().map((row) => {
              const month = months[row.index];
              if (month === undefined) {
                return null;
              }
              const future = calendarMonthIndex(month) > todayIdx;
              return (
                <div
                  key={row.key}
                  data-index={row.index}
                  ref={virtualizer.measureElement}
                  className="absolute left-0 w-full"
                  style={{
                    transform: `translateY(${row.start}px)`,
                    paddingBottom: row.index === months.length - 1 ? 0 : 32,
                  }}
                >
                  <section className={cn(future && 'opacity-50')}>
                    <h3 className="px-6 text-xl font-semibold leading-6 text-content">
                      {MONTH_LABELS[month.month0]}, {month.year}
                    </h3>
                    <RangeMonthGrid
                      year={month.year}
                      month0={month.month0}
                      draft={draft}
                      today={today}
                      onPick={(day) => setDraft(pickIsoRange(draft, day))}
                    />
                  </section>
                </div>
              );
            })}
        </div>
      </div>

      {/* «Выбрать» активна только при выборе (#670): пустой черновик
          подтверждать нечего. «Сбросить» — рядом, когда потребителю нужен
          возврат к состоянию «без периода» (канон пары: secondary слева,
          primary справа, как в WizardBottomBar потоков). */}
      <StickyBottomBar>
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
            aria-current={day === today ? 'date' : undefined}
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
