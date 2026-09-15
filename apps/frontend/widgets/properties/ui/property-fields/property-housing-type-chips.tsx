'use client';

import type { JSX } from 'react';
import type { PropertyType } from '@/entities/property';
import { isApartmentCategory, propertyHousingTypeOptions } from '@/features/properties';
import { Button } from '@/shared/ui/design';

/**
 * Чипы «Тип жилья» — общая часть визарда создания (шаг 3, Figma
 * 1218:54295) и формы правки (карта #583, тикет #590). Показываются для
 * категории «Квартира» (квартира↔апартаменты); «Студии» в макетах нет в
 * домене (типов 10) — опции берутся из фичи (решение консистентности).
 */
export type PropertyHousingTypeChipsProps = {
  readonly type: PropertyType;
  readonly onChange: (type: PropertyType) => void;
};

export function PropertyHousingTypeChips({
  type,
  onChange,
}: PropertyHousingTypeChipsProps): JSX.Element | null {
  if (!isApartmentCategory(type)) {
    return null;
  }
  return (
    <div className="flex flex-col gap-2">
      <span className="text-base font-medium leading-[18px] text-content">Тип жилья</span>
      <div role="group" aria-label="Тип жилья" className="flex flex-wrap gap-2">
        {propertyHousingTypeOptions.map((option) => {
          const selected = type === option.value;
          return (
            <Button
              key={option.value}
              // Внутри <form> формы правки кнопка без type = submit.
              type="button"
              variant="secondary"
              size="small"
              className="rounded-button"
              selected={selected}
              aria-pressed={selected}
              onClick={() => onChange(option.value)}
            >
              {option.label}
            </Button>
          );
        })}
      </div>
    </div>
  );
}
