'use client';

import { useState } from 'react';
import type { JSX, ReactNode } from 'react';
import { Check, SmallArrowDown } from '@/shared/assets/icons';
import { cn } from '@/shared/lib/cn';
import { ListRow } from './list-row';
import { Modal, ModalContent } from './modal';
import { filterPickerOptions, pickerOptionByValue } from './picker-filter';
import { SearchField } from './search-field';

/** Пикер-поле дизайн-слоя (#505; первый потребитель — «Привязанный объект»
 * формы контакта, Figma 1281:48439 — закрытый триггер). Триггер — бокс поля
 * 56px, как у TextField titleOut: значение 16/18 слева, шеврон справа; без
 * значения — плейсхолдер. Список открывается в адаптивном Modal (карточка
 * ≥768px / нижний шит уже), как MonthYearPicker: поповер-дропдаун на
 * мобильном проигрывает шиту по эргономике, поэтому новый Radix-пакет
 * (Popover/Select) не подключается. В шите — поиск по названию и подсказке
 * (регистронезависимо, filterPickerOptions), опции ListRow с галкой у
 * выбранной, при clearable — строка очистки сверху (выбор даёт
 * onValueChange(null), «без объекта»); пустой результат — emptyLabel.
 * Выбор закрывает пикер, поиск сбрасывается. Фокус-кольца у триггера-поля
 * нет (как у полей ввода, решение владельца 2026-08-26); опции — обычные
 * кнопки, клавиатура — Tab/Enter/Space + фокус-менеджмент Modal. */
export type PickerOption = {
  readonly value: string;
  readonly label: string;
  readonly hint?: string;
  readonly icon?: ReactNode;
};

export type PickerFieldProps = {
  /** Заголовок над боксом; он же — заголовок шита. */
  readonly title?: string;
  readonly placeholder?: string;
  /** Значение — value выбранной опции; null — очищено («без объекта»). */
  readonly value?: string | null;
  readonly options: ReadonlyArray<PickerOption>;
  /** Строка очистки в шите (clearLabel, по умолчанию «Без объекта»). */
  readonly clearable?: boolean;
  readonly clearLabel?: string;
  /** Поиск по списку (по умолчанию включён). */
  readonly searchable?: boolean;
  readonly searchPlaceholder?: string;
  readonly emptyLabel?: string;
  readonly error?: string;
  readonly disabled?: boolean;
  readonly onValueChange?: (value: string | null) => void;
};

export function PickerField({
  title,
  placeholder,
  value,
  options,
  clearable = false,
  clearLabel = 'Без объекта',
  searchable = true,
  searchPlaceholder = 'Поиск',
  emptyLabel = 'Ничего не найдено',
  error,
  disabled = false,
  onValueChange,
}: PickerFieldProps): JSX.Element {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState('');
  const selected = pickerOptionByValue(options, value);
  const visible = filterPickerOptions(options, query);

  const handleOpenChange = (next: boolean): void => {
    setOpen(next);
    if (!next) {
      setQuery('');
    }
  };

  const handleSelect = (optionValue: string): void => {
    handleOpenChange(false);
    onValueChange?.(optionValue);
  };

  const handleClear = (): void => {
    handleOpenChange(false);
    onValueChange?.(null);
  };

  const sheetTitle = title ?? placeholder ?? '';

  return (
    <div className={cn('flex w-full flex-col gap-2 font-sans', disabled && 'opacity-50')}>
      {title !== undefined && (
        <span className="text-base font-medium leading-[18px] text-content">{title}</span>
      )}
      <button
        type="button"
        aria-haspopup="dialog"
        aria-expanded={open}
        disabled={disabled}
        onClick={() => handleOpenChange(true)}
        className={cn(
          'flex h-14 w-full cursor-pointer items-center rounded-button bg-surface-muted py-0 pl-[18px] pr-2 text-left transition-shadow',
          !disabled && error === undefined && 'hover:shadow-[inset_0_0_0_2px_var(--dl-input-border)]',
          error !== undefined && 'bg-surface-danger hover:shadow-none',
        )}
      >
        <span
          className={cn(
            'min-w-0 flex-1 truncate text-base leading-[18px]',
            selected === undefined ? 'text-content-secondary' : 'text-content',
          )}
        >
          {selected !== undefined ? selected.label : (placeholder ?? '')}
        </span>
        <span
          className="flex h-11 w-11 shrink-0 items-center justify-center text-content-tertiary"
          aria-hidden
        >
          <SmallArrowDown className="h-4 w-4" />
        </span>
      </button>
      {error !== undefined && <span className="text-[13px] leading-[15px] text-error">{error}</span>}
      <Modal open={open} onOpenChange={handleOpenChange}>
        <ModalContent title={sheetTitle} titleSrOnly={title === undefined}>
          <div className="flex flex-col gap-3">
            {searchable && (
              <SearchField
                placeholder={searchPlaceholder}
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                onClear={() => setQuery('')}
              />
            )}
            <div className="-mx-6 flex flex-col">
              {clearable && <ListRow title={clearLabel} onSelect={handleClear} />}
              {visible.map((option) => (
                <ListRow
                  key={option.value}
                  leading={option.icon}
                  title={option.label}
                  subtitle={option.hint}
                  trailing={
                    option.value === value ? <Check className="h-6 w-6 text-primary" /> : undefined
                  }
                  onSelect={() => handleSelect(option.value)}
                />
              ))}
              {visible.length === 0 && (
                <p className="px-6 py-4 text-sm text-content-tertiary">{emptyLabel}</p>
              )}
            </div>
          </div>
        </ModalContent>
      </Modal>
    </div>
  );
}
