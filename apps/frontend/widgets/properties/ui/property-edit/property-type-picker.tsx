'use client';

import { useState } from 'react';
import type { JSX } from 'react';
import type { PropertyType } from '@/entities/property';
import { propertyCategoryOf, propertyCategoryOptions } from '@/features/properties';
import { SmallArrowDown } from '@/shared/assets/icons';
import { cn } from '@/shared/lib/cn';
import { ChipButton, Modal, ModalContent } from '@/shared/ui/design';

/**
 * Поле «Тип объекта» формы правки (карта #583, тикет #590; триггер —
 * анатомия PickerField, Figma 1550:95852, шит — 1554:97471): закрытый
 * бокс поля 56px со значением и шевроном; список открывается в
 * адаптивном Modal, в шите — чипы-пилюли тех же 9 категорий, что на
 * шаге 1 создания (канон владельца #1081, кадры 1213-52111 + 1218-54295):
 * апартаменты и студия чипом не выбираются — уточнение
 * квартира/апартаменты/студия живёт в поле «Тип жилья» формы. Выбор
 * закрывает шит.
 */
export type PropertyTypePickerProps = {
  readonly title?: string;
  readonly placeholder?: string;
  readonly value?: PropertyType;
  readonly error?: string;
  readonly onValueChange?: (type: PropertyType) => void;
};

export function PropertyTypePicker({
  title,
  placeholder,
  value,
  error,
  onValueChange,
}: PropertyTypePickerProps): JSX.Element {
  const [open, setOpen] = useState(false);
  // Двухуровневая модель: типы жилья (квартира/апартаменты/студия)
  // показывает категория «Квартира» — и в закрытом боксе, и отметкой
  // чипа в шите; сам тип остаётся в значении поля (меняет «Тип жилья»).
  const selectedCategory = value === undefined ? undefined : propertyCategoryOf(value);
  const selected = propertyCategoryOptions.find((option) => option.value === selectedCategory);
  const sheetTitle = title ?? placeholder ?? '';

  const handleSelect = (next: PropertyType): void => {
    setOpen(false);
    onValueChange?.(next);
  };

  return (
    <div className={cn('flex w-full flex-col gap-2 font-sans')}>
      {title !== undefined && (
        <span className="text-base font-medium leading-[18px] text-content">{title}</span>
      )}
      <button
        type="button"
        aria-haspopup="dialog"
        aria-expanded={open}
        onClick={() => setOpen(true)}
        className={cn(
          'flex h-14 w-full cursor-pointer items-center rounded-button bg-surface-muted py-0 pl-[18px] pr-2 text-left transition-shadow',
          error === undefined
            ? 'hover:shadow-[inset_0_0_0_2px_var(--dl-input-border)]'
            : 'bg-surface-danger hover:shadow-none',
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
      <Modal open={open} onOpenChange={setOpen}>
        {/* Серая метка 16/18 вместо канонического H3 — документированный
            в ModalContent вариант «макеты с серой меткой» (#499). */}
        <ModalContent
          title={sheetTitle}
          titleClassName="text-base font-medium text-content-tertiary"
        >
          <div role="group" aria-label={sheetTitle} className="flex flex-wrap gap-2">
            {propertyCategoryOptions.map((option) => (
              <ChipButton
                key={option.value}
                selected={option.value === selectedCategory}
                onClick={() => handleSelect(option.value)}
              >
                {option.label}
              </ChipButton>
            ))}
          </div>
        </ModalContent>
      </Modal>
    </div>
  );
}
