import { useState } from 'react';
import type { ReactNode } from 'react';
import type { JSX } from 'react';
import type { PaymentType } from '@/entities/payment';
import { TYPE_LABELS } from '@/features/payments';
import {
  AmountField,
  groupedAmount,
  sanitizeAmountInput,
  syncAmountInputDom,
  TextField,
} from '@/shared/ui/design';
import {
  kopecksToAmountInputString,
  parseRublesToKopecks,
} from '@/shared/lib/format-money';

/**
 * Общий хром шагов визарда создания платежа (#464): заголовок шага
 * (Figma Heading 699:8717 — H3 20/24 + подзаголовок 14/16), нижняя
 * панель действия над StickyBottomBar, подсказка открытого поиска
 * (Figma 1049:46256 — иллюстрация 128 + текст 16/18) и денежное поле
 * шага суммы (карта #1005) — компонентами делится визард операции.
 */

export function WizardHeading({
  title,
  subtitle,
}: {
  readonly title: string;
  readonly subtitle?: string;
}): JSX.Element {
  return (
    <div className="flex flex-col gap-2 px-6 pt-6">
      <h1 className="m-0 text-xl font-semibold leading-6 text-content">{title}</h1>
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
        src="/images/payments/category-search.png"
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

/** Денежное поле шага суммы визардов — два яруса (карта #1005, решение
 * владельца 01.10; макеты 1858:104557/105397 мобилка, 2913:69551 широкий,
 * DESIGN.md §11): <1024 (мобилка и планшет) — крупный дисплей «0 ₽» по
 * центру (канон AmountField: скрытый focusable input с цифровой
 * клавиатурой ОС), ≥1024 (ПК) — компактный бокс 56px Title In (маска
 * суммы — sanitizeAmountInput + syncAmountInputDom, как в MoneyField
 * аренды). Контракт — копейки (целые, строго положительные): компонент
 * держит «сырой» буфер набранного — дисплей обоих ярусов, пока поле в
 * фокусе, — иначе эхо копеек переписывало бы бокс под курсором («25,» →
 * «25», «25,5» → «25,50» — блокировка ввода, канон поля правки платежа
 * #467). Вне фокуса значение следует за пропсом: восстановление черновика
 * и нормализация после blur («25,5» → «25,50» — правило экрана). Подгонка
 * состояния при рендере — официальный паттерн React (тот же, что в
 * AmountField). Ярусы разводятся display:none — вне дерева доступности
 * остаётся один input «Сумма». Проп focusOnMount — подъём клавиатуры
 * маунтом в задаче жеста (#1151): доезжает только до дисплейного яруса
 * <1024, ПК-бокс не фокусируется никогда (п.4 #1151 — ПК не трогаем). */
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
  /** Фокус дисплея при маунте — только для маунтов внутри жеста (#1151). */
  readonly focusOnMount?: boolean;
}): JSX.Element {
  const kopecksToRaw = (): string =>
    kopecks === undefined ? '' : kopecksToAmountInputString(kopecks);
  // Сырой буфер набранного: источник отрисовки, пока поле в фокусе
  // (фокус-трекинг бокса; дисплейный AmountField буферизует сам). Без
  // гейта эхо копеек переписывало бы бокс под курсором.
  const [buffer, setBuffer] = useState(kopecksToRaw);
  const [boxFocused, setBoxFocused] = useState(false);
  const [syncedKopecks, setSyncedKopecks] = useState(kopecks);
  if (!boxFocused && kopecks !== syncedKopecks) {
    setSyncedKopecks(kopecks);
    setBuffer(kopecksToRaw());
  }

  const change = (sanitized: string): void => {
    setBuffer(sanitized);
    onKopecksChange(parseRublesToKopecks(sanitized, { positive: true }));
  };

  return (
    <>
      <div className="hidden w-full desktop:block">
        <TextField
          variant="titleIn"
          title={label}
          type="text"
          inputMode="decimal"
          autoComplete="off"
          spellCheck={false}
          value={groupedAmount(buffer)}
          onFocus={() => setBoxFocused(true)}
          onBlur={() => setBoxFocused(false)}
          onChange={(event) => {
            const sanitized = sanitizeAmountInput(event.target.value);
            change(sanitized);
            syncAmountInputDom(event.target, sanitized);
          }}
        />
      </div>
      <AmountField
        className="desktop:hidden"
        value={buffer}
        onChange={change}
        label={label}
        focusOnMount={focusOnMount}
      />
    </>
  );
}

const SEGMENT_ORDER = ['expense', 'income'] as const;

/** Сегмент «Расход/Доход» шага суммы (Figma 1858:104562) — один для обоих
 * визардов: серый контейнер 232px на дисплейном ярусе (<1024), во всю
 * колонку на ПК (2913:69741); выбранный — белая пилюля с тенью. Подписи —
 * TYPE_LABELS («Расход»/«Доход»), направление выбрано явно подсветкой;
 * видимое значение (пресет входа или дефолт «Доход») вычисляет вызывающий
 * шаг и отдаёт готовым type. */
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
      className="flex w-full max-w-[232px] rounded-2xl bg-surface-muted p-[2px] desktop:max-w-none"
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
              'min-h-10 flex-1 cursor-pointer rounded-[14px] text-sm font-medium leading-4 transition-colors outline-none focus-visible:ring-4 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-surface '
              + (selected
                ? 'bg-surface text-content shadow-[0_2px_4px_rgba(0,0,0,0.16)]'
                : 'text-content-secondary')
            }
          >
            {TYPE_LABELS[option]}
          </button>
        );
      })}
    </div>
  );
}
