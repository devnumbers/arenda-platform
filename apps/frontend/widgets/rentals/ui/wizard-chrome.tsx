import { useState } from 'react';
import type { ComponentPropsWithRef, JSX, ReactNode } from 'react';
import { ArrowLeft, Cancel, SmallArrowDown } from '@/shared/assets/icons';
import type { RentalPaymentDay, RentalUtilities } from '@/entities/rental';
import { cn } from '@/shared/lib/cn';
import {
  Button,
  Checkbox,
  groupedAmount,
  IconButton,
  ListRow,
  MonthDaysGrid,
  PageContent,
  PickerMenu,
  type PickerMenuGroup,
  sanitizeAmountInput,
  StickyBottomBar,
  Switch,
  syncAmountInputDom,
  TopNav,
  TopNavTitle,
} from '@/shared/ui/design';
import { paymentDayFromPicker, UTILITIES_OPTIONS, utilitiesLabel } from '@/features/rentals';

/**
 * Общий хром арендных форм (#530/#532): заголовок шага — Mobile/Heading/H1
 * 28/32 из макета (1270:46904 — крупнее, чем H3-заголовки визарда
 * платежей), нижняя панель действия над StickyBottomBar, поля «Input Field»
 * (1270:46905/47386): хвостовые иконки — канон IconButton primary (круг 44,
 * hover-подложка), обязательные поля — красная звёздочка (решение владельца
 * 2026-09-05). Здесь же канонные куски, общие визарду создания (#530) и
 * правке условий (#532): пикер дня оплаты, поле коммуналки, строка
 * тумблера автоплатежа.
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
 * ховере бокса). Открывает пикер-поверхность снаружи (onClick). Бокс
 * работает и триггером PickerMenu (asChild): остальные пропсы кнопки —
 * обработчики/aria/ref от Radix — прокидываются на внутреннюю <button>. */
export function PickerTriggerBox({
  title,
  required = false,
  value,
  placeholder,
  icon,
  onClick,
  error,
  className,
  ...rest
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
} & Omit<
  ComponentPropsWithRef<'button'>,
  'title' | 'type' | 'onClick' | 'className' | 'aria-label'
>): JSX.Element {
  return (
    <div className={cn('flex w-full flex-col gap-2 font-sans', className)}>
      <FieldTitle title={title} required={required} />
      <button
        type="button"
        onClick={onClick}
        aria-label={`${title}: ${value ?? placeholder}`}
        className="group/trigger flex h-14 w-full cursor-pointer items-center rounded-button bg-surface-muted py-0 pl-[18px] pr-2 text-left transition-shadow outline-none hover:shadow-[inset_0_0_0_2px_var(--dl-input-border)]"
        {...rest}
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

/** Компактное денежное поле (решения владельца 2026-09-05): бокс 56px
 * без символа рубля — при вводе живая группировка разрядов «56 000»,
 * плейсхолдера нет (пустое поле пустое); очистка — круглая Cancel-иконка
 * справа. Паттерн поля правки платежа #467 (AmountBoxInput): «сырое»
 * значение + groupedAmount + syncAmountInputDom. */
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

  return (
    <div className="flex w-full flex-col gap-2 font-sans">
      <FieldTitle title={title} required={required} />
      <div className="flex h-14 w-full items-center rounded-button bg-surface-muted pl-[18px] pr-2 transition-shadow hover:shadow-[inset_0_0_0_2px_var(--dl-input-border)]">
        <input
          type="text"
          inputMode="decimal"
          autoComplete="off"
          spellCheck={false}
          aria-label={ariaLabel}
          value={groupedAmount(raw)}
          onChange={(event) => {
            const sanitized = sanitizeAmountInput(event.target.value);
            onRawChange(sanitized);
            syncAmountInputDom(event.target, sanitized);
          }}
          className="h-full min-w-0 flex-1 border-none bg-transparent text-base leading-[18px] text-content outline-none"
        />
        {hasValue && (
          <IconButton
            icon={<Cancel />}
            label={`Очистить «${title}»`}
            variant="secondary"
            onClick={onClear}
          />
        )}
      </div>
    </div>
  );
}

/** Полноэкранный оверлей выбора дня оплаты (общий для визарда #530 и правки
 * условий #532; поверхность «временный пикер поверх формы»): одиночный
 * выбор числа либо «последний день месяца» — они взаимоисключимы, тап по
 * выбранному числу снимает его. */
export function PaymentDayPicker({
  initial,
  onClose,
  onConfirm,
}: {
  readonly initial: RentalPaymentDay | undefined;
  readonly onClose: () => void;
  readonly onConfirm: (paymentDay: RentalPaymentDay | undefined) => void;
}): JSX.Element {
  const [draftDay, setDraftDay] = useState<number | undefined>(
    typeof initial === 'number' ? initial : undefined,
  );
  const [draftLast, setDraftLast] = useState<boolean>(initial === 'last');

  const handleDay = (day: number): void => {
    setDraftLast(false);
    setDraftDay((prev) => (prev === day ? undefined : day));
  };

  const handleLast = (): void => {
    if (draftLast) {
      setDraftLast(false);
      return;
    }
    setDraftLast(true);
    setDraftDay(undefined);
  };

  const ready = draftLast || draftDay !== undefined;

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label="Выбор дня оплаты"
      className="fixed inset-0 z-50 flex flex-col bg-surface"
    >
      <TopNav
        leading={
          <IconButton icon={<ArrowLeft />} label="Назад" onClick={onClose} />
        }
      >
        <TopNavTitle title="Выберите день" />
      </TopNav>
      <PageContent className="flex h-[calc(100dvh-72px)] flex-col pb-0">
        <div className="flex-1 min-h-0 overflow-y-auto pb-6">
          <div className="pt-6">
            <MonthDaysGrid
              days={30}
              selectedDays={
                draftDay === undefined || draftLast ? undefined : new Set([draftDay])
              }
              onDayToggle={handleDay}
            />
          </div>
          <div className="pt-6">
            {/* Строка — сам переключатель (Enter/Space/клик по ListRow);
                чекбокс — его зрительный индикатор, некликабельный: тап
                не должен тонуть дважды (строка + чекбокс). */}
            <ListRow
              title="Последний день месяца"
              onSelect={handleLast}
              trailing={
                <Checkbox
                  checked={draftLast}
                  tabIndex={-1}
                  aria-hidden
                  className="pointer-events-none"
                />
              }
            />
          </div>
        </div>
        {/* Футер только с черновиком выбора (решение владельца 2026-09-05:
            скрытие вместо дизейбла); на планшете тянется с шитом. */}
        {ready && (
          <StickyBottomBar fullWidthContent>
            <WizardBottomBar>
              <Button
                className="w-full"
                onClick={() => onConfirm(paymentDayFromPicker({ day: draftDay, last: draftLast }))}
              >
                Выбрать
              </Button>
            </WizardBottomBar>
          </StickyBottomBar>
        )}
      </PageContent>
    </div>
  );
}

/** Поле режима коммунальных платежей (общее для шага условий #530 и правки
 * #532) на общем PickerSelectField. */
export function UtilitiesPickerField({
  value,
  onChange,
}: {
  readonly value: RentalUtilities;
  readonly onChange: (utilities: RentalUtilities) => void;
}): JSX.Element {
  return (
    <PickerSelectField
      title="Коммунальные платежи"
      valueLabel={utilitiesLabel(value)}
      groups={[
        {
          options: UTILITIES_OPTIONS.map((option) => ({
            label: option.label,
            selected: value === option.value,
            onSelect: () => onChange(option.value),
          })),
        },
      ]}
    />
  );
}

/** Поле-селект одной опции — общая анатомия «Input Field» макета
 * (1270:46821): заголовок над триггер-боксом 56px с выбранной меткой и
 * шевроном; тап открывает канонный PickerMenu (меню на десктопе, шит на
 * мобиле), выбор применяется сразу. Общее для коммуналки (#530/#532) и
 * «За сколько напоминать» (#826). Собрано на общем PickerTriggerBox —
 * обёртку и заголовок поля рисует он сам. */
export function PickerSelectField({
  title,
  valueLabel,
  groups,
}: {
  readonly title: string;
  /** Метка выбранной опции в триггере. */
  readonly valueLabel: string;
  readonly groups: ReadonlyArray<PickerMenuGroup>;
}): JSX.Element {
  return (
    <PickerMenu title={title} groups={groups}>
      <PickerTriggerBox
        title={title}
        value={valueLabel}
        placeholder={valueLabel}
        icon={<SmallArrowDown className="h-6 w-6" />}
        onClick={() => {}}
      />
    </PickerMenu>
  );
}

/** Строка тумблера «Сделать платеж автоматическим?» (общая для шага
 * настроек #530 и правки #532; опечатки подписи макета не воспроизводятся):
 * включённый автоплатёж фиксирует оплату в назначенный день сам,
 * выключенный оставляет отметку владельцу. */
export function AutoPayRow({
  checked,
  onCheckedChange,
}: {
  readonly checked: boolean;
  readonly onCheckedChange: (checked: boolean) => void;
}): JSX.Element {
  return (
    <div className="flex items-center justify-between gap-4">
      <div className="flex min-w-0 flex-col gap-1">
        <span className="text-base font-medium leading-[18px] text-content">
          Сделать платеж автоматическим?
        </span>
        <span className="text-sm leading-4 text-content-tertiary">
          Оплата аренды автоматически зафиксируется в назначенный день.
          Либо отмечайте её вручную, а мы пришлём напоминание
        </span>
      </div>
      <Switch
        checked={checked}
        onCheckedChange={(value) => onCheckedChange(value === true)}
        aria-label="Сделать платеж автоматическим"
      />
    </div>
  );
}
