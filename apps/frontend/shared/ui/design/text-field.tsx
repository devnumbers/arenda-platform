'use client';

import { useId, useLayoutEffect, useRef, useState } from 'react';
import type { ComponentProps, FocusEvent, JSX, MouseEvent, ReactNode } from 'react';
import { Cancel } from '@/shared/assets/icons';
import { cn } from '@/shared/lib/cn';
import { IconButton } from './icon-button';

/** Поле ввода дизайн-слоя (Figma 948:46646, «Input Field»): бокс фиксированной
 * высоты 56px в обоих вариантах. Title Out — заголовок над боксом, строка по
 * центру. Title In — плавающий лейбл: в покое плейсхолдер (16px) стоит по
 * вертикальному центру бокса (Default: alignItems center), при фокусе или
 * непустом значении уменьшается до 13px и уходит наверх; строка значения —
 * 27..45 (Typing: 10 + лейбл 15 + 2 + значение 18).
 * Обязательное поле — красная звёздочка сразу после заголовка в обоих
 * вариантах (проп required; решение владельца 2026-09-05, визард аренды).
 * Состояния: Error (красный бокс; Title In — текст ошибки в слоте лейбла
 * красным 13/15 над значением, 948:46954; Title Out — строкой под боксом),
 * Limited (счётчик красным),
 * Disabled (opacity 0.5), Hover (inset-обводка 2px) — фокус-кольца у поля
 * нет намеренно (решение владельца 2026-08-26): видимый признак фокуса —
 * каретка. Кнопка очистки появляется при непустом значении и переданном
 * onClear; остаётся в таб-порядке.
 * Инлайновая единица измерения (Figma 1218:54304 — характеристики,
 * 1270:46905 — арендная плата; решение владельца 2026-10-22; референс
 * поведения — инпут цены Яндекс.Аренды) — проп inlineSuffix, декоративный
 * текст (aria-hidden), который является частью строки значения: следует
 * за введённым текстом, у пустого поля стоит у левого края. Цвет
 * (аменда владельца 2026-10-22): серый, пока поле пусто и не в фокусе;
 * при фокусе или непустом значении — цветом значения. Механика без
 * JS-замера: невидимый двойник текста (тот же value, та же типографика,
 * в потоке) занимает ровно ширину значения, инпут лежит поверх
 * абсолютом; спан единицы стоит в потоке следом. Клик по пустой части
 * бокса фокусирует инпут с кареткой в конце. Только для однострочного
 * поля и без плейсхолдера (плейсхолдер не помещается в
 * сжатый инпут); прежние каноны сняты: префикс-единица слева до ввода
 * (1218:54295) и правый статичный суффикс.
 * Многострочное поле (Figma 1227:58065 «Title Out Multi Lines») — textarea
 * в том же боксе: бокс растёт от контента, минимум 56px; вариант titleIn
 * с multiline не сочетается — плавающий лейбл рассчитан на одну строку.
 * autoGrow — опциональное авторасширение: поле тянется по контенту от 3
 * строк до 200px, дальше скролл внутри (комментарий задачи, решение
 * владельца 2026-09-03); без него textarea фиксированная — высота в
 * строках задаётся пропом rows (3 по умолчанию; статичные 8 строк
 * описания объекта — решение владельца 2026-10-02, кадр 1218:54295),
 * текст сверх — скроллится внутри (нативное поведение textarea). */

export type TextFieldVariant = 'titleOut' | 'titleIn';

type TextFieldBaseProps = {
  readonly variant?: TextFieldVariant;
  readonly title?: string;
  /** Обязательное поле: после заголовка рисуется красная звёздочка
   * (решение владельца 2026-09-05, визард аренды). Визуальный маркер —
   * обязательность проверяет форма (error), не атрибут required. */
  readonly required?: boolean;
  readonly description?: string;
  readonly error?: string;
  /** Лимит символов: нативный maxLength инпута (ввод сверх запрещён) +
   * счётчик «длина/лимит»; красным — при достижении лимита. */
  readonly maxLength?: number;
  readonly onClear?: () => void;
  /** Инлайновая единица измерения (м², м, ₽): декоративный текст,
   * следующий за введённым значением; у пустого поля — у левого края.
   * Цвет: серый (content-secondary), пока поле пусто и не в фокусе;
   * при фокусе или непустом значении — цветом значения (content;
   * аменда владельца 2026-10-22).
   * Только однострочные поля с инпутом в потоке (titleOut или без
   * заголовка; titleIn с плавающим лейблом не поддержан); плейсхолдер в
   * этом режиме игнорируется — сжатому инпуту некуда его класть
   * (механика — невидимый двойник текста в ветке unitActive). */
  readonly inlineSuffix?: ReactNode;
  /** Только с multiline: статичная высота поля в строках (3 по умолчанию);
   * текст сверх — скролл внутри, поле не растёт (описание объекта — 8
   * строк, решение владельца 2026-10-02, кадр 1218:54295). */
  readonly rows?: number;
  /** Только с multiline: поле растёт по контенту от 3 строк до ~10
   * (200px), дальше — скролл внутри; при очистке сжимается обратно. */
  readonly autoGrow?: boolean;
};

/** Однострочное поле — пропсы input; многострочное (multiline) — пропсы
 * textarea: обработчики получают события соответствующего элемента. */
export type TextFieldProps = TextFieldBaseProps &
  (
    | ({ readonly multiline?: false } & Omit<ComponentProps<'input'>, 'size' | 'maxLength'>)
    | ({ readonly multiline: true } & Omit<ComponentProps<'textarea'>, 'maxLength' | 'ref' | 'rows'>)
  );

/** Показатели счётчика и ошибок — кегль Figma Mobile/Text/S (13/15). */

export function TextField({
  className,
  variant = 'titleOut',
  title,
  required = false,
  description,
  error,
  maxLength,
  onClear,
  inlineSuffix,
  multiline = false,
  autoGrow = false,
  rows: rowsCount = 3,
  value,
  disabled,
  placeholder,
  ...props
}: TextFieldProps): JSX.Element {
  const inputId = useId();
  const textareaRef = useRef<HTMLTextAreaElement | null>(null);
  // Инлайновая единица: ширину инпута задаёт невидимый двойник текста
  // (тот же текст той же типографикой в потоке) — единица следует за
  // введённым без всякого JS-замера. Только ветки с инпутом в потоке
  // (titleOut или без заголовка): в titleIn с плавающим лейблом ветки
  // нет — там единица не поддержана. mousedown по освободившейся части
  // бокса фокусирует инпут (ищется запросом по боксу: caller-ref инпута
  // не трогаем). Клик по самому инпуту не перехватывается — он
  // позиционирует каретку; клик мимо ставит каретку в конец значения,
  // готовый к дописыванию.
  const unitActive =
    inlineSuffix !== undefined
    && !multiline
    && (variant === 'titleOut' || title === undefined);
  const unitValue =
    typeof value === 'string' || typeof value === 'number' ? String(value) : '';
  // Цвет единицы (аменда владельца 2026-10-22): серый, пока поле пусто и
  // не в фокусе; при фокусе или непустом значении — цветом значения.
  const [unitFocused, setUnitFocused] = useState(false);
  const unitTinted = unitFocused || unitValue.length > 0;
  const focusInlineUnit = (event: MouseEvent<HTMLDivElement>): void => {
    const input = event.currentTarget.querySelector<HTMLInputElement>('input');
    if (input === null || event.target === input) {
      return;
    }
    // preventDefault: mousedown по умолчанию унёс бы фокус с инпута.
    event.preventDefault();
    input.focus();
    // Каретка в конец — только у текстовых типов: у number/email движок
    // браузера бросает InvalidStateError на принудительную селекцию.
    if (input.type === 'text') {
      const end = input.value.length;
      input.setSelectionRange(end, end);
    }
  };
  // Общая анатомия однострочного инпута для обеих веток (обычной и
  // инлайновой единицы); в режиме единицы плейсхолдер игнорируется —
  // сжатому до ширины текста инпуту некуда его класть. Фокус-обработчики
  // единицы составлены с caller-обработчиками, не заменяют их.
  const renderSingleLineInput = (inputClassName: string): JSX.Element => {
    const { onFocus: callerFocus, onBlur: callerBlur, ...inputRest } =
      props as ComponentProps<'input'>;
    const focusProps = unitActive
      ? {
          onFocus: (event: FocusEvent<HTMLInputElement>) => {
            callerFocus?.(event);
            setUnitFocused(true);
          },
          onBlur: (event: FocusEvent<HTMLInputElement>) => {
            callerBlur?.(event);
            setUnitFocused(false);
          },
        }
      : {};
    return (
      <input
        id={inputId}
        className={inputClassName}
        disabled={disabled}
        value={value}
        placeholder={unitActive ? undefined : placeholder}
        maxLength={maxLength}
        {...inputRest}
        {...focusProps}
      />
    );
  };
  /** Заголовок с маркером обязательности — красная звёздочка следом. */
  const titleNode =
    title !== undefined && required ? (
      <>
        {title} <span aria-hidden className="text-error">*</span>
      </>
    ) : (
      title
    );

  // Авторасширение multiline-поля: высота подгоняется под контент на
  // каждое изменение значения; потолок — CSS max-h (скролл внутри).
  useLayoutEffect(() => {
    const el = textareaRef.current;
    if (!el || !autoGrow) return;
    el.style.height = 'auto';
    el.style.height = `${el.scrollHeight}px`;
  }, [autoGrow, value]);
  const hasValue = typeof value === 'string' && value.length > 0;
  const showClear = onClear !== undefined && hasValue && !disabled;
  const counter =
    maxLength !== undefined && typeof value === 'string'
      ? `${value.length}/${maxLength}`
      : undefined;
  const counterDanger = maxLength !== undefined && typeof value === 'string' && value.length >= maxLength;
  // Title In + Error (макет 948:46954, сверка экрана кода Т5 #1102): текст
  // ошибки занимает слот лейбла — красный 13/15 над значением внутри бокса;
  // строкой под полем он не рисуется (это стейт Title Out). Ошибка возможна
  // только после попытки — поле «в typing», верхнее положение фиксированное.
  // Слот лейбла есть только при переданном title: без него ошибка уходит
  // в нижнюю строку, как у Title Out (иначе терялась бы вовсе).
  const errorInLabel = variant === 'titleIn' && !multiline && title !== undefined && error !== undefined;
  const bottomLeft = errorInLabel ? description : (error ?? description);

  const box = cn(
    'flex w-full items-center rounded-button bg-surface-muted pl-[18px] pr-2 transition-shadow',
    multiline ? 'min-h-14 flex-col justify-center py-[10px]' : 'h-14 py-0',
    unitActive && 'cursor-text',
    !disabled && error === undefined && 'hover:shadow-[inset_0_0_0_2px_var(--dl-input-border)]',
    error !== undefined && 'bg-surface-danger hover:shadow-none',
  );
  const input = cn(
    'w-full min-w-0 border-none bg-transparent text-base leading-[18px] text-content outline-none placeholder:text-content-secondary',
    // Каретка — часть канона поля: в макетах Cursor рисуется синим 2px
    // (Color/Blue #2b7fff, 948:46869), не системным чёрным (сверка экрана
    // кода, Т5 #1102).
    'caret-primary',
    multiline ? 'resize-none' : 'h-full',
  );

  return (
    <div className={cn('flex w-full flex-col gap-2 font-sans', disabled && 'opacity-50', className)}>
      {variant === 'titleOut' && titleNode !== undefined && (
        <label htmlFor={inputId} className="text-base font-medium leading-[18px] text-content">
          {titleNode}
        </label>
      )}
      <div
        className={box}
        onMouseDown={unitActive && !disabled ? focusInlineUnit : undefined}
      >
        {variant === 'titleIn' && !multiline && title !== undefined && (
          <div className="relative h-full min-w-0 flex-1">
            {/* Инпут идёт перед лейблом: peer-варианты требуют, чтобы peer
                предшествовал цели (~). Строка значения заперта на 27..45
                (pt-27 + lh-18 + pb-11 = 56), как раскладка Typing в Figma:
                10 + лейбл 15 + 2 + значение 18. Лейбл — плавающий: в покое
                плейсхолдер стоит по вертикальному центру бокса (Default:
                alignItems center), при фокусе/значении уменьшается до 13px
                и уходит наверх (10px). Плейсхолдер инпута прозрачен: его
                роль играет лейбл. */}
            <input
              id={inputId}
              className="peer h-full w-full border-none bg-transparent pb-[11px] pt-[27px] text-base leading-[18px] text-content caret-primary outline-none placeholder:text-transparent"
              disabled={disabled}
              value={value}
              placeholder={title}
              maxLength={maxLength}
              // Лейбл при ошибке заменён текстом ошибки — доступное имя
              // держит aria-label, а ошибка связана с полем через
              // aria-describedby + aria-invalid (иначе скринридер узнаёт
              // о красной строке только фактом её в дереве).
              aria-invalid={error !== undefined ? true : undefined}
              aria-label={errorInLabel ? title : undefined}
              aria-describedby={errorInLabel ? `${inputId}-error` : undefined}
              {...(props as ComponentProps<'input'>)}
            />
            {errorInLabel ? (
              <span id={`${inputId}-error`} className="pointer-events-none absolute left-0 top-[10px] text-[13px] leading-[15px] text-error">
                {error}
              </span>
            ) : (
              <label
                htmlFor={inputId}
                className={cn(
                  'pointer-events-none absolute left-0 text-content-secondary transition-[top,font-size,line-height] duration-300 motion-reduce:duration-150',
                  'top-[19px] text-base leading-[18px]',
                  'peer-focus:top-[10px] peer-focus:text-[13px] peer-focus:leading-[15px]',
                  'peer-[:not(:placeholder-shown)]:top-[10px] peer-[:not(:placeholder-shown)]:text-[13px] peer-[:not(:placeholder-shown)]:leading-[15px]',
                )}
              >
                {titleNode}
              </label>
            )}
          </div>
        )}
        {(variant === 'titleOut' || title === undefined || multiline) && (
          <>
            {multiline ? (
              <textarea
                id={inputId}
                ref={textareaRef}
                className={cn(input, 'min-h-[54px]', autoGrow && 'max-h-[200px] overflow-y-auto')}
                disabled={disabled}
                value={value}
                placeholder={placeholder}
                maxLength={maxLength}
                rows={rowsCount}
                {...(props as ComponentProps<'textarea'>)}
              />
            ) : unitActive ? (
              // Режим инлайновой единицы: невидимый двойник текста (тот же
              // текст, та же типографика) занимает ровно ширину значения в
              // потоке, инпут лежит поверх абсолютом — каретка, выделение и
              // ввод обычные, синхронизация с двойником не нужна (это один
              // и тот же value в одном рендере).
              <span className="relative inline-flex h-full min-w-[2px] max-w-full items-center overflow-hidden">
                <span aria-hidden className="invisible whitespace-pre text-base leading-[18px]">
                  {unitValue}
                </span>
                {renderSingleLineInput(cn(input, 'absolute inset-0 px-0'))}
              </span>
            ) : (
              renderSingleLineInput(input)
            )}
            {unitActive && (
              <span
                aria-hidden
                className={cn(
                  'shrink-0 pl-1 text-base leading-[18px]',
                  unitTinted ? 'text-content' : 'text-content-secondary',
                )}
              >
                {inlineSuffix}
              </span>
            )}
          </>
        )}
        {showClear && (
          <IconButton
            icon={<Cancel />}
            label={title !== undefined ? `Очистить «${title}»` : 'Очистить поле'}
            variant={error !== undefined ? 'danger' : 'secondary'}
            disabled={disabled}
            onClick={onClear}
            // В обычном режиме инпут w-full сам прижимает кнопку; ml-auto
            // нужен только в режиме единицы, где инпут сжат до ширины текста.
            className={unitActive ? 'ml-auto' : undefined}
          />
        )}
      </div>
      {(bottomLeft !== undefined || counter !== undefined) && (
        <div className="flex items-center justify-between gap-2 text-[13px] leading-[15px]">
          {bottomLeft !== undefined && (
            // Красит фактическое содержимое: при ошибке в слоте лейбла здесь
            // живёт description (hint) и остаётся серым — красным может быть
            // только ошибка.
            <span className={cn(!errorInLabel && error !== undefined && bottomLeft === error ? 'text-error' : 'text-content-tertiary')}>
              {bottomLeft}
            </span>
          )}
          {counter !== undefined && (
            <span className={cn(counterDanger ? 'text-error' : 'text-content-tertiary')}>{counter}</span>
          )}
        </div>
      )}
    </div>
  );
}
