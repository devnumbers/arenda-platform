import { useCallback, useState } from 'react';
import type { ChangeEvent, ReactNode, Ref } from 'react';
import type { JSX } from 'react';
import type { PaymentType } from '@/entities/payment';
import { TYPE_LABELS } from '@/features/payments';
import {
  groupedAmount,
  sanitizeAmountInput,
  syncAmountInputDom,
  TextField,
} from '@/shared/ui/design';
import {
  kopecksToAmountInputString,
  parseRublesToKopecks,
} from '@/shared/lib/format-money';

/** Порог ПК-яруса — зеркало `--breakpoint-desktop` (app/globals.css):
 * маунт-фокус суммы живёт только на ≥1024 (мобилка и планшет — нет). */
const DESKTOP_MIN_WIDTH = 1024;

/** Решение маунт-фокуса суммы: жестовые маунты (#1151) фокусируются
 * всегда — так iOS поднимает клавиатуру из жеста (research #1148 §A);
 * остальные — только на ПК (решение владельца 2026-10-22: инпут на ПК
 * активен сразу; на мобилке вне жеста клавиатуру платформа всё равно
 * не поднимает, фокус без клавиатуры не даёт ничего). */
export function shouldFocusAmountOnMount(
  gestureFocus: boolean,
  isDesktopViewport: boolean,
): boolean {
  return gestureFocus || isDesktopViewport;
}

/**
 * Общий хром шагов визарда создания платежа (#464): заголовок шага
 * (Figma Heading 699:8717 — H3 20/24 + подзаголовок; вариант 'h1' —
 * обновлённый хедер H1/600 Onest SemiBold 28/32 платёжных поверхностей:
 * выбор категории — спека research #1149, Figma 1049:34768, тикет #1152;
 * шаг периодичности платежа и его ветки (с подзаголовком) — макеты
 * 3213:71290/71312/71350, тикет #1192; категория операции — тоже h1
 * (решение владельца 13.10, разворот решения 06.10 «H3 операции»),
 * остальные шаги операции — прежний H3), нижняя панель действия над
 * StickyBottomBar, подсказка
 * открытого поиска (Figma 1049:46256 — иллюстрация 128 + текст 16/18) и
 * денежное поле шага суммы (карта #1005) — компонентами делится визард
 * операции.
 */

export function WizardHeading({
  title,
  subtitle,
  variant = 'h3',
}: {
  readonly title: string;
  readonly subtitle?: string;
  /** 'h1' — обновлённый хедер платёжных страниц: Service/Heading/H1/600
   * 28/32 (text-2xl в этом проекте = 28/32, app/globals.css); дефолт 'h3'
   * — прежние заголовки шагов 20/24. Подзаголовок допустим у любого
   * варианта (ветки периодичности 3213:71312/71350 несут его под H1). */
  readonly variant?: 'h1' | 'h3';
}): JSX.Element {
  const headingClass =
    variant === 'h1'
      ? 'text-2xl font-semibold leading-8 text-content'
      : 'text-xl font-semibold leading-6 text-content';
  return (
    <div className="flex flex-col gap-2 px-6 pt-6">
      <h1 className={'m-0 ' + headingClass}>{title}</h1>
      {subtitle !== undefined && (
        <p className="text-sm leading-4 text-content-secondary">{subtitle}</p>
      )}
    </div>
  );
}

/** Контент шага при открытом поиске категорий: вместо списка — иллюстрация
 * с одним текстом (пустой запрос — «Начните искать», без совпадений —
 * «Ничего не нашлось»); создание категории здесь не упоминается. */
export function CategorySearchHint({ text }: { readonly text: string }): JSX.Element {
  return (
    <div className="flex flex-col items-center gap-4 pt-16">
      <img
        src="/images/payments/category-search.webp"
        alt=""
        width={128}
        height={128}
        className="h-32 w-32"
      />
      <p className="max-w-[320px] text-center text-base leading-[18px] text-content-secondary">
        {text}
      </p>
    </div>
  );
}

export function WizardBottomBar({ children }: { readonly children: ReactNode }): JSX.Element {
  // Без горизонтального паддинга: панель всегда внутри контейнера, где
  // 24px уже есть (StickyBottomBar p-6, экран успеха px-6) — иначе кнопка
  // уже контента (двойные 48px). Без нижнего тоже: у StickyBottomBar свой
  // 24px + safe-area.
  return <div className="flex flex-col gap-3">{children}</div>;
}

/** Денежное поле шага суммы визардов — единый ярус (решение владельца
 * 2026-10-22, макет 2913:69551 «Mobile» повторяет ПК-канон): бокс 56px
 * Title In на всех ширинах, крупный дисплей «0 ₽» выведен из визардов
 * (разворот двухъярусной карты #1005; макеты 1858:104557/105397 больше
 * не действуют). Маска суммы —
 * sanitizeAmountInput + syncAmountInputDom, как в MoneyField аренды.
 * Контракт — копейки (целые, строго положительные): компонент держит
 * «сырой» буфер набранного, пока поле в фокусе, — иначе эхо копеек
 * переписывало бы бокс под курсором («25,» → «25», «25,5» → «25,50» —
 * блокировка ввода, канон поля правки платежа #467). Вне фокуса значение
 * следует за пропсом: восстановление черновика и нормализация после blur
 * («25,5» → «25,50» — правило экрана). Подгонка состояния при рендере —
 * официальный паттерн React. Атрибуты inputMode/pattern/enterKeyHint —
 * контракт мобильной клавиатуры бывшего дисплейного яруса (цифровой
 * блок, «Готово» закрывает клавиатуру). Маунт-фокус — shouldFocusAmountOnMount:
 * ПК ≥1024 — инпут активен сразу (разворот п.4 #1151 «ПК не трогаем»),
 * жестовые маунты платежа (#1151) — всегда, мобилка вне жеста — без
 * фокуса. */
export type WizardAmountFieldViewProps = {
  /** Сырой буфер набранного — источник отрисовки бокса. */
  readonly buffer: string;
  /** Ref инпута (маунт-фокус, вьюха его только навешивает). */
  readonly mountFocusRef?: Ref<HTMLInputElement>;
  readonly onInputChange: (event: ChangeEvent<HTMLInputElement>) => void;
  readonly onFocus: () => void;
  readonly onBlur: () => void;
  readonly label: string;
};

/** Stateless-вьюха бокса: держит только дерево элементов — node-канон
 * тестов (канон button.test.ts: компонент без хуков вызывается как
 * функция и инспектируется). */
export function WizardAmountFieldView({
  buffer,
  mountFocusRef,
  onInputChange,
  onFocus,
  onBlur,
  label,
}: WizardAmountFieldViewProps): JSX.Element {
  return (
    <TextField
      ref={mountFocusRef}
      variant="titleIn"
      title={label}
      type="text"
      inputMode="decimal"
      enterKeyHint="done"
      pattern="[0-9]*"
      autoComplete="off"
      spellCheck={false}
      value={groupedAmount(buffer)}
      onFocus={onFocus}
      onBlur={onBlur}
      onChange={onInputChange}
    />
  );
}

export function WizardAmountField({
  label,
  kopecks,
  onKopecksChange,
  focusOnMount,
}: {
  readonly label: string;
  /** Копейки; undefined — ещё не задана. */
  readonly kopecks: number | undefined;
  readonly onKopecksChange: (kopecks: number | undefined) => void;
  /** Безусловный фокус при маунте — только для маунтов внутри жеста
   * (#1151); остальные ярусы решает shouldFocusAmountOnMount. */
  readonly focusOnMount?: boolean;
}): JSX.Element {
  const kopecksToRaw = (): string =>
    kopecks === undefined ? '' : kopecksToAmountInputString(kopecks);
  // Сырой буфер набранного: источник отрисовки, пока поле в фокусе.
  // Без гейта эхо копеек переписывало бы бокс под курсором.
  const [buffer, setBuffer] = useState(kopecksToRaw);
  const [boxFocused, setBoxFocused] = useState(false);
  const [syncedKopecks, setSyncedKopecks] = useState(kopecks);
  if (!boxFocused && kopecks !== syncedKopecks) {
    setSyncedKopecks(kopecks);
    setBuffer(kopecksToRaw());
  }

  // Маунт-фокус в callback-ref (не autoFocus): колбэк вызывается синхронно
  // в фазе коммита, который при flushSync-переходе остаётся в задаче жеста
  // клика — так iOS поднимает клавиатуру (#1151). Ручной useCallback вместо
  // автормемо компилятора: колбэк утекает в React (ref-слот) — канон
  // CODING_STANDARDS «values escaping to non-React code». Зависимость
  // focusOnMount держит идентичность стабильной между рендерами.
  // Ширина вьюпорта читается на маунте: рефокус при пересечении 1024
  // ресайзом не нужен никому.
  const mountFocusRef = useCallback(
    (node: HTMLInputElement | null): void => {
      if (node === null) {
        return;
      }
      const isDesktopViewport = window.matchMedia(
        `(min-width: ${DESKTOP_MIN_WIDTH}px)`,
      ).matches;
      if (shouldFocusAmountOnMount(focusOnMount === true, isDesktopViewport)) {
        node.focus();
      }
    },
    [focusOnMount],
  );

  const change = (sanitized: string): void => {
    setBuffer(sanitized);
    onKopecksChange(parseRublesToKopecks(sanitized, { positive: true }));
  };

  return (
    <WizardAmountFieldView
      buffer={buffer}
      mountFocusRef={mountFocusRef}
      onInputChange={(event) => {
        const sanitized = sanitizeAmountInput(event.target.value);
        change(sanitized);
        syncAmountInputDom(event.target, sanitized);
      }}
      onFocus={() => setBoxFocused(true)}
      onBlur={() => setBoxFocused(false)}
      label={label}
    />
  );
}

const SEGMENT_ORDER = ['expense', 'income'] as const;

/** Сегмент «Расход/Доход» шага суммы (Figma 2913:69741 — решение
 * владельца 2026-10-22, отмена 232px дисплейного яруса #1005) — один для
 * обоих визардов, во всю ширину на всех ярусах; форма правки платежа
 * (#1197) получает ту же ширину без отдельного пропа (fullWidth снесён).
 * Тень выбранной пилюли обрезается серым треком (контейнер overflow-clip),
 * между пилюлями зазор 2px — как в макете. Подписи — TYPE_LABELS
 * («Расход»/«Доход»), направление выбрано явно подсветкой; видимое
 * значение (пресет входа или дефолт «Доход») вычисляет вызывающий шаг
 * и отдаёт готовым type. */
export function WizardDirectionSegment({
  type,
  onTypeChange,
  ariaLabel,
}: {
  /** Направление, видимое на сегменте (явный выбор или пресет/дефолт). */
  readonly type: PaymentType;
  readonly onTypeChange: (type: PaymentType) => void;
  readonly ariaLabel: string;
}): JSX.Element {
  return (
    <div
      role="radiogroup"
      aria-label={ariaLabel}
      className="flex w-full gap-[2px] overflow-clip rounded-2xl bg-surface-muted p-[2px]"
    >
      {SEGMENT_ORDER.map((option) => {
        const selected = type === option;
        return (
          <button
            key={option}
            type="button"
            role="radio"
            aria-checked={selected}
            onClick={() => onTypeChange(option)}
            className={
              // focus-ring 2px без offset: рисуется в 2px-гуттере трека и
              // не клипается его overflow-clip (канон ParticipantRoleSegmented
              // при том же клипе); ring-4 с offset клипался бы целиком.
              'min-h-10 flex-1 cursor-pointer rounded-[14px] text-sm font-medium leading-4 transition-colors outline-none focus-visible:ring-2 focus-visible:ring-primary '
              + (selected
                ? 'bg-surface text-content shadow-[0_2px_4px_rgba(0,0,0,0.16)]'
                // Радио-диалект: невыбранный темнеет шагами трека
                // (прецедент ParticipantRoleSegmented), выбранный — без шагов.
                : 'text-content-secondary hover:bg-surface-muted-hover active:bg-surface-muted-active')
            }
          >
            {TYPE_LABELS[option]}
          </button>
        );
      })}
    </div>
  );
}
