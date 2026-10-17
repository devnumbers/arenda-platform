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
  rangeFeedWindow,
  settleIsoRange,
} from '@/shared/lib/calendar';
import { formatRangeBound } from '@/shared/lib/date-format';
import { useBodyScrollLock } from '@/shared/lib/hooks/useBodyScrollLock';
import { createPrependGate, flingChunks } from './calendar-feed-gate';
import {
  type PrependAnchorCapture,
  planPrependRestore,
} from './calendar-prepend-anchor';
import {
  calendarMonthGridRows,
  calendarMonthIntrinsicPx,
} from './calendar-month-metrics';
import { MONTH_LABELS, daysInMonth, firstWeekdayOfMonth, WEEKDAY_LABELS } from '@/shared/ui/design/month-grid';

/** SSR-безопасный layout-эффект: запись позиции до пейнта (планы 049/051). */
const useIsomorphicLayoutEffect =
  typeof window === 'undefined' ? useEffect : useLayoutEffect;

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

/** Прокрутка внутренней ленты к месяцу — только самой лентой, без
 * scrollIntoView: тот реквизирует всех скролл-предков цели, и Chromium
 * сдвигает и viewport документа (замер 16.10: открытие пикера уводило
 * документ на сотни пикселей; под локом overflow-y: hidden документ
 * программно скроллируем, и утечка стала видимой — карта #1437, план
 * 041). Цель — offsetTop секции относительно ленты (лента position:
 * relative — её offsetParent): надёжно прямо в ref-колбэке до первой
 * отрисовки, когда rect контента за content-visibility ещё не развит
 * (rect-математика вырождалась в ноль и ломала якорный скролл
 * открытия — догонка плана 042). block: 'start'. */
function scrollListToMonth(list: HTMLElement | null, month: HTMLElement | undefined): void {
  if (list === null || month === undefined) {
    return;
  }
  list.scrollTop = month.offsetTop;
}

/** Блок месяца ленты (мемо, план 050): приписка перерисовывает только
 * новые блоки — существующие выпадают из реконсиляции, и тяжесть прохода
 * перестаёт расти с глубиной ленты (раньше каждая приписка пересчитывала
 * все блоки: Date+ISO и дизейблы на каждую ячейку — к глубине пары лет
 * это ~5–6k ячеек за проход). Пропсы — примитивы и стабильные колбэки:
 * перерендер блока только когда меняются его выбор/границы (тап-уровень).
 * Оценка высоты офскрина — точная (calendar-month-metrics по ширине
 * колонки, план 051): с ручным якорем приписки расхождение статической
 * оценки с реальностью больше никто не компенсирует. */
const CalendarMonthBlock = memo(function CalendarMonthBlock({
  year,
  month0,
  isFirst,
  beyondMaxDate,
  today,
  draft,
  value,
  minDate,
  maxDate,
  allowPast,
  columnWidth,
  registerRef,
  onSelect,
}: {
  readonly year: number;
  readonly month0: number;
  readonly isFirst: boolean;
  readonly beyondMaxDate: boolean;
  readonly today: IsoDate;
  readonly draft: IsoDate | null;
  readonly value: IsoDate | null;
  readonly minDate: IsoDate | undefined;
  readonly maxDate: IsoDate | undefined;
  readonly allowPast: boolean;
  /** Ширина колонки ленты (ResizeObserver родителя) — точная оценка
   * офскрин-высоты (план 051); null — ещё не замерена, фолбэк 360. */
  readonly columnWidth: number | null;
  readonly registerRef: (key: string, node: HTMLElement | null) => void;
  readonly onSelect: (iso: IsoDate, selected: boolean) => void;
}): JSX.Element {
  const days = daysInMonth(year, month0);
  const leadingBlanks = firstWeekdayOfMonth(year, month0);
  const key = monthKey(year, month0);
  // Оценка высоты офскрина — точная (calendar-month-metrics): с ручным
  // якорем приписки ([overflow-anchor:none]) некомпенсированный дрейф
  // оценки при первом рендере блока дёргал бы контент на десятки px.
  const rows = calendarMonthGridRows(days, leadingBlanks);
  return (
    <section
      ref={(node) => {
        registerRef(key, node);
      }}
      style={{
        containIntrinsicSize: `auto ${calendarMonthIntrinsicPx({ rows, columnWidth })}px`,
      }}
      className={cn(
        isFirst ? undefined : 'mt-8',
        beyondMaxDate && 'opacity-50',
        '[content-visibility:auto]',
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
  const monthRefs = useRef(new Map<string, HTMLElement>());
  const scrollRef = useRef<HTMLDivElement | null>(null);
  // Прыжок за дорисованный край: лента дорисовывается порцией, цель
  // скроллится после монтирования новых месяцев. Ключ абсолютный — старт
  // ленты уезжает назад при дорисовке.
  const pendingScrollKey = useRef<string | null>(null);
  // Гейт приписки (план 050): одна порция в полёте — скролл-события
  // одного кадра не суммируются в один DOM-коммит (на телефоне такие
  // коммиты были паузами кадра 150–600 мс прямо в моментуме).
  const [prependGate] = useState(createPrependGate);
  // Ручной якорь приписки (план 051): захват {ключ первого месяца,
  // offsetTop} перед припиской — восстановление позиции до пейнта в
  // layout-эффекте ниже. Нативный scroll anchoring выключен
  // ([overflow-anchor:none] на ленте): в WebKit его не было до Safari 27,
  // и каждая приписка телепортировала контент на высоту порции.
  const prependAnchorRef = useRef<PrependAnchorCapture | null>(null);
  // Ширина колонки — вход точной оценки офскрин-высоты (план 051).
  const contentRef = useRef<HTMLDivElement | null>(null);
  const [columnWidth, setColumnWidth] = useState<number | null>(null);
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
  // Стабильная регистрация ref'а блока месяца — проп мемо-блока.
  const setMonthRef = useCallback((key: string, node: HTMLElement | null) => {
    if (node !== null) {
      monthRefs.current.set(key, node);
    } else {
      monthRefs.current.delete(key);
    }
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

  // Открытие — сразу к месяцу якоря (значение или ближайший доступный
  // день): лента стартует порцией раньше якоря, без прыжка первый кадр
  // показывал бы не его. Layout-эффект — до отрисовки, вспышки чужого
  // месяца нет. Один раз: после дорисовки назад старт уплывает —
  // повторного прыжка к якорю не нужно.
  const anchorScrolledRef = useRef(false);
  useLayoutEffect(() => {
    if (anchorScrolledRef.current || anchorIndex <= 0) {
      return;
    }
    anchorScrolledRef.current = true;
    const target = calendarMonthOfIndex(startIndex + anchorIndex);
    scrollListToMonth(scrollRef.current, monthRefs.current.get(monthKey(target.year, target.month0)));
  }, [anchorIndex, startIndex]);

  // Дорисовка прыжка за край — после монтирования новых месяцев.
  useEffect(() => {
    // Приписка доехала до DOM — гейт открывает следующий запрос (план 050).
    prependGate.settled();
    const key = pendingScrollKey.current;
    if (key === null) {
      return;
    }
    pendingScrollKey.current = null;
    scrollListToMonth(scrollRef.current, monthRefs.current.get(key));
  }, [monthsCount, firstMonth, prependGate]);

  // Ручной якорь приписки (план 051): до пейнта возвращаем позицию,
  // сдвинутую DOM-вставой порции над вьюпортом — тот же приём, что починил
  // колесо (план 049). Дельта offsetTop того же элемента точна при любых
  // оценках высот; пишем в текущий scrollTop — моментум между скролл-
  // событием и коммитом сохраняется. Захват ставится только в
  // prepend-ветке handleScroll: прыжки чипа скроллятся к цели своим
  // эффектом выше, аппенды внизу позицию не сдвигают.
  useIsomorphicLayoutEffect(() => {
    const captured = prependAnchorRef.current;
    prependAnchorRef.current = null;
    const list = scrollRef.current;
    if (captured === null || list === null) {
      return;
    }
    const el = monthRefs.current.get(captured.key);
    const inserted = planPrependRestore({
      anchorTop: el?.offsetTop ?? null,
      capturedTop: captured.top,
    });
    if (inserted !== null) {
      list.scrollTop += inserted;
    }
  }, [monthsCount, firstMonth]);

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
  // порцию месяцев назад. DOM растёт только пройденной глубиной, а тяжёлый
  // layout оффскрина снимает content-visibility на блоках месяцев.
  // Сдвиг позиции при prepend держит ручной якорь до пейнта (план 051,
  // layout-эффект выше): ставка 042 на нативный scroll anchoring работала
  // только на Chromium — в WebKit нативного якоря нет (overflow-anchor —
  // Safari 27+), и на телефоне каждая приписка телепортировала контент на
  // высоту порции; на ленте [overflow-anchor:none] — компенсация ровно
  // одна, наша, на всех движках. Порог 3000 держит триггер далеко от
  // стены, а под фликом порций ставится несколько — по скорости прокрутки
  // (flingChunks, кэп 2 — план 051): под душащим CPU scroll-события
  // доезжают с лагом в сотни мс (замер 16.10: 7 событий на 90 000px
  // флика), и одна порция за событие не поспевает за моментумом.
  const lastScrollRef = useRef<{ st: number; t: number } | null>(null);
  const handleScroll = (): void => {
    const list = scrollRef.current;
    if (list === null) return;
    const now = performance.now();
    const previous = lastScrollRef.current;
    lastScrollRef.current = { st: list.scrollTop, t: now };
    if (endlessPast && list.scrollTop <= PREPEND_THRESHOLD_PX) {
      const chunks = flingChunks(previous, now, list.scrollTop, FEED_CHUNK_PX);
      // Склейка по кадру (план 050): скролл-события одного кадра не
      // суммируют порции в один DOM-коммит — пока предыдущая приписка в
      // полёте, запросы склеиваются с ней; settled() зовёт эффект
      // рендера приписки.
      const portion = prependGate.request(chunks);
      if (portion === null) {
        return;
      }
      // Захват якоря до приписки (план 051): layout-эффект после коммита
      // вернёт позицию до пейнта — сдвиг порции не закрашивается.
      const capturedKey = monthKey(firstMonth.year, firstMonth.month0);
      const capturedEl = monthRefs.current.get(capturedKey);
      prependAnchorRef.current =
        capturedEl !== undefined ? { key: capturedKey, top: capturedEl.offsetTop } : null;
      setFirstMonth((month) =>
        calendarMonthOfIndex(calendarMonthIndex(month) - CALENDAR_FEED_PREPEND_CHUNK * portion),
      );
      setMonthsCount((count) => count + CALENDAR_FEED_PREPEND_CHUNK * portion);
      return;
    }
    if (
      forwardEdge === null
      && list.scrollTop + list.clientHeight >= list.scrollHeight - APPEND_THRESHOLD_PX
    ) {
      setMonthsCount((count) => count + CALENDAR_FEED_MONTHS);
    }
  };

  /** Прыжок по чипу: месяц ленты — скроллом, за дорисованный край —
   * дорисовка порцией и скролл после монтирования. Вперёд лента растёт
   * годом; назад — только в безпольном режиме, порциями; при закрытом
   * будущем вперёд прыжок ограничен краем (колесо годов и так не предложит
   * месяц за maxDate). */
  const jumpTo = (year: number, month0: number): void => {
    const targetIdx = calendarMonthIndex({ year, month0 });
    if (targetIdx >= startIndex && targetIdx - startIndex < monthsCount) {
      scrollListToMonth(scrollRef.current, monthRefs.current.get(monthKey(year, month0)));
      return;
    }
    pendingScrollKey.current = monthKey(year, month0);
    if (targetIdx < startIndex) {
      let nextStart = startIndex;
      while (nextStart > targetIdx) {
        nextStart -= CALENDAR_FEED_PREPEND_CHUNK;
      }
      setMonthsCount((count) => count + (startIndex - nextStart));
      setFirstMonth(calendarMonthOfIndex(nextStart));
      return;
    }
    setMonthsCount(
      forwardEdge !== null
        ? calendarMonthIndex(forwardEdge) - startIndex + 1
        : targetIdx - startIndex + CALENDAR_FEED_MONTHS,
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

      {/* Позицию при дорисовке держит ручной якорь до пейнта (план 051):
          layout-эффект возвращает scrollTop в тот же кадр, что DOM-встав.
          [overflow-anchor:none] выключает нативный scroll anchoring:
          в WebKit его не было до Safari 27 (телефон владельца — без него),
          и каждая приписка телепортировала контент; с ручным якорем
          нативная компенсация удвоила бы сдвиг на Chromium. */}
      <div
        ref={scrollRef}
        onScroll={handleScroll}
        className="relative min-h-0 flex-1 overflow-y-auto overscroll-contain [scrollbar-width:none] [overflow-anchor:none]"
      >
        <div ref={contentRef} className="mx-auto w-full max-w-column pb-[136px]">
          {months.map(({ year, month0 }, index) => {
            // Приглушённое будущее за месяцем maxDate (жёсткий край #1394):
            // контекст после черты виден, но выбирать там нечего — 1:1 с
            // пикером периода.
            const beyondMaxDate =
              forwardEdge !== null
              && maxDate !== undefined
              && calendarMonthIndex({ year, month0 })
                > calendarMonthIndex(calendarMonthOf(maxDate));
            return (
              <CalendarMonthBlock
                key={monthKey(year, month0)}
                year={year}
                month0={month0}
                isFirst={index === 0}
                beyondMaxDate={beyondMaxDate}
                today={today}
                draft={draft}
                value={value}
                minDate={minDate}
                maxDate={maxDate}
                allowPast={allowPast}
                columnWidth={columnWidth}
                registerRef={setMonthRef}
                onSelect={selectDraft}
              />
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

  const monthRefs = useRef(new Map<string, HTMLElement>());
  const scrollRef = useRef<HTMLDivElement | null>(null);
  // Темп прокрутки для скоростно-зависимого prepend (flingChunks).
  const lastRangeScrollRef = useRef<{ st: number; t: number } | null>(null);
  // Гейт приписки (план 050, дотянут до range планом 051): одна порция в
  // полёте — скролл-события кадра не складываются в один коммит; без гейта
  // антистенная приёмка здесь держалась на склейке очереди событий в
  // монструозный коммит из сотен месяцев — телефонные паузы в моментуме.
  const [rangePrependGate] = useState(createPrependGate);
  // Ручной якорь приписки (план 051) — как у одиночного пикера: нативного
  // scroll anchoring в WebKit нет (Safari 27+), ручная компенсация до
  // пейнта единственная.
  const rangePrependAnchorRef = useRef<PrependAnchorCapture | null>(null);
  // Месяц прыжка чипа за дорисованный назад край — скроллится после
  // монтирования новых месяцев.
  const pendingJumpRef = useRef<string | undefined>(undefined);
  // Первый рендер — сразу к месяцу конца применённого диапазона.
  const scrolledRef = useRef(false);

  useLayoutEffect(() => {
    const jump = pendingJumpRef.current;
    if (jump !== undefined) {
      pendingJumpRef.current = undefined;
      scrollListToMonth(scrollRef.current, monthRefs.current.get(jump));
    }
  });

  // Ручной якорь приписки (план 051): до пейнта возвращаем позицию,
  // сдвинутую DOM-вставой порции, — дельта offsetTop того же элемента.
  useIsomorphicLayoutEffect(() => {
    const captured = rangePrependAnchorRef.current;
    rangePrependAnchorRef.current = null;
    const list = scrollRef.current;
    if (captured === null || list === null) {
      return;
    }
    const el = monthRefs.current.get(captured.key);
    const inserted = planPrependRestore({
      anchorTop: el?.offsetTop ?? null,
      capturedTop: captured.top,
    });
    if (inserted !== null) {
      list.scrollTop += inserted;
    }
  }, [firstMonth]);

  // Приписка доехала до DOM — гейт открывает следующий запрос (план 050).
  useEffect(() => {
    rangePrependGate.settled();
  }, [firstMonth, rangePrependGate]);

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
    const chunks = flingChunks(previous, now, container.scrollTop, FEED_CHUNK_PX);
    const portion = rangePrependGate.request(chunks);
    if (portion === null) {
      return;
    }
    // Захват якоря до приписки (план 051).
    const capturedKey = monthKey(firstMonth.year, firstMonth.month0);
    const capturedEl = monthRefs.current.get(capturedKey);
    rangePrependAnchorRef.current =
      capturedEl !== undefined ? { key: capturedKey, top: capturedEl.offsetTop } : null;
    setFirstMonth((month) =>
      calendarMonthOfIndex(calendarMonthIndex(month) - RANGE_PREPEND_CHUNK * portion),
    );
  };

  /** Прыжок по чипу: месяц ленты — скроллом; глубже дорисованного края —
   * дорисовка порциями назад и скролл после монтирования. */
  const jumpTo = (year: number, month0: number): void => {
    const targetIdx = calendarMonthIndex({ year, month0 });
    if (targetIdx >= calendarMonthIndex(firstMonth)) {
      scrollListToMonth(scrollRef.current, monthRefs.current.get(monthKey(year, month0)));
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
      // Ref-колбэки секций диспатчатся раньше ref самой ленты (дети
      // раньше родителя): scrollRef.current здесь ещё null, и якорь
      // терялся молча (регресс плана 042, пойман приёмкой #1428).
      // Микротаска выходит из коммита до отрисовки — ref'ы на месте,
      // вспышки чужого месяца нет.
      queueMicrotask(() => scrollListToMonth(scrollRef.current, node));
    }
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

      {/* Позицию при дорисовке держит ручной якорь до пейнта (план 051):
          layout-эффект возвращает scrollTop в тот же кадр, что DOM-встав.
          Прежняя ставка на нативный scroll anchoring (план 042) работала
          только на Chromium: в WebKit якоря нет (overflow-anchor —
          Safari 27+), и приписка телепортировала контент; ручная
          компенсация снята была зря. [overflow-anchor:none] держит одну
          компенсацию (нашу) на всех движках. */}
      <div
        ref={scrollRef}
        onScroll={handleScroll}
        className="relative min-h-0 flex-1 overflow-y-auto overscroll-contain [scrollbar-width:none] [overflow-anchor:none]"
      >
        <div className="mx-auto w-full max-w-column pb-[136px]">
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
