'use client';

import type { JSX } from 'react';
import type { PropertyType } from '@/entities/property';
import { propertyCategoryOptions } from '@/features/properties';
import { ChipButton } from '@/shared/ui/design';

/** Шаг 1 «Выберите, какая у вас недвижимость» (Figma 1213-52111): чипы
 * категорий переносятся на новую строку (flex-wrap, зазор 8). Кнопки
 * продолжения в макете нет — выбор категории сразу переводит на шаг
 * адреса; выбранное состояние пригождается при возврате «назад». */

export type CategoryStepProps = {
  readonly selected?: PropertyType;
  readonly onSelect: (type: PropertyType) => void;
};

export function CategoryStep({ selected, onSelect }: CategoryStepProps): JSX.Element {
  return (
    <div role="group" aria-label="Категория объекта" className="mt-6 flex flex-wrap gap-2 px-6">
      {propertyCategoryOptions.map((option) => (
        <ChipButton
          key={option.value}
          selected={selected === option.value}
          onClick={() => onSelect(option.value)}
        >
          {option.label}
        </ChipButton>
      ))}
    </div>
  );
}
