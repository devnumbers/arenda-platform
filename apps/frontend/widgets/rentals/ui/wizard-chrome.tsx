import type { ReactNode } from 'react';
import type { JSX } from 'react';
import { Cancel } from '@/shared/assets/icons';
import { cn } from '@/shared/lib/cn';
import {
  groupedAmount,
  IconButton,
  sanitizeAmountInput,
  syncAmountInputDom,
} from '@/shared/ui/design';

/**
 * Общий хром шагов визарда создания аренды (#530): заголовок шага —
 * Mobile/Heading/H1 28/32 из макета (1270:46904 — крупнее, чем H3-заголовки
 * визарда платежей), нижняя панель действия над StickyBottomBar, поля
 * «Input Field» (1270:46905/47386): хвостовые иконки — канон IconButton
 * primary (круг 44, hover-подложка), обязательные поля — красная
 * звёздочка (решение владельца 2026-09-05).
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
      <h1 className="m-0 font-sans text-[28px] font-semibold leading-8 text-content">{title}</h1>
      {subtitle !== undefined && (
        <p className="text-sm leading-4 text-content-secondary">{subtitle}</p>
      )}
    </div>
  );
}

export function WizardBottomBar({ children }: { readonly children: ReactNode }): JSX.Element {
  // Без горизонтального паддинга: панель всегда внутри контейнера, где
  // 24px уже есть (StickyBottomBar p-6) — иначе кнопка уже контента.
  return <div className="flex flex-col gap-3">{children}</div>;
}

/** Заголовок поля с маркером обязательности — та же анатомия, что у
 * канонного TextField с required: красная звёздочка следом. */
export function FieldTitle({
  title,
  required = false,
}: {
  readonly title: string;
  readonly required?: boolean;
}): JSX.Element {
  return (
    <span className="text-base font-medium leading-[18px] text-content">
      {title}
      {required && (
        <>
          {' '}
          <span aria-hidden className="text-error">*</span>
        </>
      )}
    </span>
  );
}

/** Триггер-бокс поля-пикера (анатомия «Input Field» из макета 1270:46821):
 * заголовок над боксом 56px, значение или плейсхолдер слева, хвостовая
 * иконка справа — канон IconButton primary (круг, hover-подложка на
 * ховере бокса). Открывает пикер-поверхность снаружи (onClick). */
export function PickerTriggerBox({
  title,
  required = false,
  value,
  placeholder,
  icon,
  onClick,
  error,
  className,
}: {
  readonly title: string;
  readonly required?: boolean;
  /** Значение; undefined — рисуется плейсхолдер приглушённым цветом. */
  readonly value: string | undefined;
  readonly placeholder: string;
  readonly icon: ReactNode;
  readonly onClick: () => void;
  readonly error?: string;
  readonly className?: string;
}): JSX.Element {
  return (
    <div className={cn('flex w-full flex-col gap-2 font-sans', className)}>
      <FieldTitle title={title} required={required} />
      <button
        type="button"
        onClick={onClick}
        aria-label={`${title}: ${value ?? placeholder}`}
        className="group/trigger flex h-14 w-full cursor-pointer items-center rounded-button bg-surface-muted py-0 pl-[18px] pr-2 text-left transition-shadow outline-none hover:shadow-[inset_0_0_0_2px_var(--dl-input-border)]"
      >
        <span
          className={cn(
            'min-w-0 flex-1 truncate text-base leading-[18px]',
            value === undefined ? 'text-content-secondary' : 'text-content',
          )}
        >
          {value ?? placeholder}
        </span>
        {/* Хвостовая иконка —IconButton-primary-анатомия: круг 44 с
            hover-подложкой, иконка #171A1C. */}
        <span
          aria-hidden
          className="flex h-11 w-11 shrink-0 items-center justify-center rounded-pill text-content transition-colors group-hover/trigger:bg-surface-muted group-active/trigger:bg-surface-muted-hover"
        >
          {icon}
        </span>
      </button>
      {error !== undefined && <span className="text-[13px] leading-[15px] text-error">{error}</span>}
    </div>
  );
}

/** Компактное денежное поле (макет 1270:46905/47386, решение владельца
 * 2026-09-05): бокс 56px с живой группировкой разрядов, «₽» прижата к
 * сумме с небольшим зазором и едет вместе с вводом (пустое поле —
 * одинокий серый «₽», заполненное — «56 000 ₽» тёмным); очистка —
 * круглая Cancel-иконка. Техника канонного AmountField: невидимый
 * измеритель задаёт ширину инпута по набранному тексту, «₽» — соседний
 * элемент (внутри value ей не место — уводит каретку и ломает
 * Backspace); «сырое» значение + syncAmountInputDom — паттерн поля
 * правки платежа (#467). */
export function MoneyField({
  title,
  required = false,
  raw,
  onRawChange,
  onClear,
  ariaLabel,
}: {
  readonly title: string;
  readonly required?: boolean;
  /** «Сырое» значение («56000», «1234,5») — источник отображения. */
  readonly raw: string;
  readonly onRawChange: (raw: string) => void;
  readonly onClear: () => void;
  readonly ariaLabel: string;
}): JSX.Element {
  const hasValue = raw.length > 0;
  const grouped = groupedAmount(raw);

  return (
    <div className="flex w-full flex-col gap-2 font-sans">
      <FieldTitle title={title} required={required} />
      <div className="flex h-14 w-full items-center rounded-button bg-surface-muted pl-[18px] pr-2 transition-shadow hover:shadow-[inset_0_0_0_2px_var(--dl-input-border)]">
        {/* обёртка инпута — ширины набранного текста (без flex-1): «₽»
            прижимается к сумме, а не к правому краю бокса */}
        <span className="relative inline-flex h-full items-center">
          {/* измеритель: ширина инпута = ширине набранного текста (пустой
              — незаметная «0», чтобы поле оставалось кликабельным) */}
          <span aria-hidden className={cn('invisible whitespace-pre px-0.5 text-base leading-[18px]', hasValue ? 'text-content' : 'text-content-tertiary')}>
            {grouped === '' ? '0' : grouped}
          </span>
          <input
            type="text"
            inputMode="decimal"
            autoComplete="off"
            spellCheck={false}
            pattern="[0-9]*"
            aria-label={ariaLabel}
            value={grouped}
            onChange={(event) => {
              const sanitized = sanitizeAmountInput(event.target.value);
              onRawChange(sanitized);
              syncAmountInputDom(event.target, sanitized);
            }}
            className="absolute inset-0 h-full w-full bg-transparent text-base leading-[18px] text-content outline-none"
          />
        </span>
        <span
          aria-hidden
          className={cn(
            'ml-2 text-base leading-[18px]',
            hasValue ? 'text-content' : 'text-content-tertiary',
          )}
        >
          ₽
        </span>
        {hasValue && (
          <IconButton
            icon={<Cancel />}
            label={`Очистить «${title}»`}
            variant="secondary"
            onClick={onClear}
            className="ml-1"
          />
        )}
      </div>
    </div>
  );
}
