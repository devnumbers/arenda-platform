'use client';

import { useCallback, useState, type ChangeEvent, type JSX, type Ref } from 'react';
import { cn } from '@/shared/lib/cn';
import { groupedAmount, sanitizeAmountInput, syncAmountInputDom } from './amount-input';

/** Поле суммы дизайн-слоя (тикет #459, Figma 834:19662 / 835:19795):
 * крупная центрированная строка «2 500 ₽» (Onest SemiBold 44/48) — правка
 * владельца 2026-08-26: ввод с обычной клавиатуры вместо numpad, только
 * цифры и запятая. Маска — sanitizeAmountInput (фильтрует символы, лимит
 * 9 999 999,99 ₽); на мобильной клавиатуре открывается цифровой блок
 * (inputMode=decimal), точка нормализуется в запятую. Значение — «сырая»
 * строка («1234,5»), в поле рисуется группировка разрядов. Фокусной
 * обводки нет намеренно (в макете фокус не отрисован); позицию ввода
 * показывает каретка цвета текста.
 *
 * Символ «₽» — снаружи инпута: суффикс-элемент рядом с цифрами (правка
 * владельца 2026-08-26: будучи частью value, он уводил каретку за себя и
 * ломал Backspace). Сам инпут скрыт под невидимым span-измерителем той же
 * строки, связка центрируется, каретка живёт среди цифр, Backspace стирает
 * по символу.
 *
 * Правка владельца 2026-08-31: пока поле в фокусе, рисуется ровно то, что
 * набирает пользователь. Родитель может эхо-возвращать нормализованное из
 * копеек значение (черновик хранит целые копейки) — его перерисовка под
 * курсором уводила каретку в конец и дописывала «,00», блокируя ввод.
 * Вне фокуса значение синхронизируется с пропсом: восстановление черновика
 * и нормализация после blur («25,5» → «25,50» — правило экрана).
 *
 * Правка #1151 (2026-10-06, research #1148 §C): вся область дисплея —
 * цель тапа. Инпут расширен на ±48px за пределы текста измерителя —
 * накрывает «₽» и поля вокруг цифр, тап в любом месте дисплея становится
 * нативным тапом по самому editable: фокус из жеста — единственный
 * поднимающий клавиатуру путь на iOS (политика WebKit, bug 195884).
 * Расширение симметричное, текст инпута центрирован в общей с измерителем
 * точке — отрисовка не смещается. Проп focusOnMount — для маунтов поля
 * в задаче жеста (переход шага визарда по «Далее», #1151): клавиатура
 * поднимается сама; маунты вне жеста (роут, восстановление черновика)
 * флаг не получают — iOS клавиатуру вне жеста всё равно не поднимает. */
export type AmountFieldProps = {
  readonly value: string;
  readonly onChange: (value: string) => void;
  readonly disabled?: boolean;
  /** Имя поля для скринридеров («Сумма»). */
  readonly label: string;
  /** Фокус инпута при маунте — только для маунтов внутри жеста (#1151). */
  readonly focusOnMount?: boolean;
  readonly className?: string;
};

export function AmountField({
  value,
  onChange,
  disabled = false,
  label,
  focusOnMount,
  className,
}: AmountFieldProps): JSX.Element {
  // Автоподъём (#1151): callback-ref фокусирует инпут при маунте — колбэк
  // вызывается синхронно в фазе коммита, который при flushSync-переходе
  // остаётся в задаче жеста клика (layout/passive-эффекты из неё уходят —
  // iOS клавиатуру не поднимет). Ручной useCallback вместо авторmemo
  // компилятора: колбэк утекает в React (ref-слот) — канон CODING_STANDARDS
  // «values escaping to non-React code». Зависимость focusOnMount держит
  // идентичность стабильной между рендерами — повторного фокуса нет.
  const focusOnMountRef = useCallback(
    (node: HTMLInputElement | null): void => {
      if (focusOnMount === true && node !== null) {
        node.focus();
      }
    },
    [focusOnMount],
  );

  // Сырой буфер набранного: источник отрисовки, пока поле в фокусе.
  const [buffer, setBuffer] = useState(value);
  const [focused, setFocused] = useState(false);
  const [syncedValue, setSyncedValue] = useState(value);
  // Вне фокуса поле следует за пропсом (восстановление черновика,
  // нормализация после blur): подгонка состояния при рендере —
  // официальный паттерн React вместо setState в эффекте.
  if (!focused && value !== syncedValue) {
    setSyncedValue(value);
    setBuffer(value);
  }

  // Управляемый input показывает маскированное значение; при вводе маска
  // вычищает лишнее — если DOM разошёлся со значением, синхронизируем его
  // вручную (React не перерисует совпавший value).
  const handleChange = (event: ChangeEvent<HTMLInputElement>): void => {
    const sanitized = sanitizeAmountInput(event.target.value);
    setBuffer(sanitized);
    onChange(sanitized);
    syncAmountInputDom(event.target, sanitized);
  };

  return (
    <AmountFieldView
      buffer={buffer}
      inputRef={focusOnMountRef}
      onInputChange={handleChange}
      onFocus={() => setFocused(true)}
      onBlur={() => setFocused(false)}
      disabled={disabled}
      label={label}
      className={className}
    />
  );
}

export type AmountFieldViewProps = {
  /** Сырой буфер набранного — источник отрисовки дисплея. */
  readonly buffer: string;
  /** Ref инпута (маунт-фокус #1151, вьюха его только навешивает). */
  readonly inputRef?: Ref<HTMLInputElement>;
  readonly onInputChange: (event: ChangeEvent<HTMLInputElement>) => void;
  readonly onFocus: () => void;
  readonly onBlur: () => void;
  readonly disabled?: boolean;
  /** Имя поля для скринридеров («Сумма»). */
  readonly label: string;
  readonly className?: string;
};

/** Stateless-вьюха дисплея: держит только дерево элементов — node-канон
 * тестов (amount-field.test.ts, канон button.test.ts: компонент без хуков
 * вызывается как функция и инспектируется). */
export function AmountFieldView({
  buffer,
  inputRef,
  onInputChange,
  onFocus,
  onBlur,
  disabled = false,
  label,
  className,
}: AmountFieldViewProps): JSX.Element {
  const amountStyle =
    'font-sans text-[2.75rem] font-semibold leading-12';

  return (
    <span
      className={cn(
        'inline-flex items-baseline justify-center',
        disabled && 'opacity-50',
        className,
      )}
    >
      <span className="relative inline-flex">
        {/* измеритель: задаёт ширину инпута по набранному тексту */}
        <span aria-hidden className={cn('invisible whitespace-pre px-0.5', amountStyle)}>
          {groupedAmount(buffer) === '' ? '0' : groupedAmount(buffer)}
        </span>
        <input
          ref={inputRef}
          type="text"
          inputMode="decimal"
          autoComplete="off"
          spellCheck={false}
          pattern="[0-9]*"
          size={1}
          aria-label={label}
          disabled={disabled}
          value={groupedAmount(buffer)}
          placeholder="0"
          onChange={onInputChange}
          onFocus={onFocus}
          onBlur={onBlur}
          className={cn(
            // -inset-x-12 — вся область дисплея тапабельна (#1151):
            // расширение на 48px за измеритель накрывает «₽» и воздух
            // вокруг цифр; расширение симметричное, центр текста инпута
            // совпадает с центром измерителя — отрисовка не едет. Крайние
            // ~24px расширения на мобайле уходят за колонку (px-6) и
            // клипаются body (overflow-x: clip) — осознанно: ядро дисплея
            // остаётся тапабельным целиком.
            'absolute inset-y-0 -inset-x-12 cursor-text bg-transparent text-center outline-none placeholder:text-content-tertiary',
            amountStyle,
            'text-content',
          )}
        />
      </span>
      <span
        aria-hidden
        className={cn(
          // ml-3 — пробел между суммой и символом рубля, как в макете «2 500 ₽»
          'ml-3',
          amountStyle,
          buffer === '' ? 'text-content-tertiary' : 'text-content',
        )}
      >
        ₽
      </span>
    </span>
  );
}
