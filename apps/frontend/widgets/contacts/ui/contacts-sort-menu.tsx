'use client';

import type { JSX, ReactNode } from 'react';
import { Check } from '@/shared/assets/icons';
import type { ContactSortOrder } from '../lib/contact-list-model';
import { Menu, MenuContent, MenuItem, MenuTrigger } from '@/shared/ui/design';

/** Меню сортировки книги контактов на десктопе (механика меню задач,
 * Figma 1603-94487): карточка 246px с опциями «Имя от А до Я» / «Имя от
 * Я до А» — книга сортируется только по имени (#508); у пункта ведущий
 * selection-квадрат (radius 8): синий с галкой у выбранного, серое кольцо
 * у остальных. Выбор сразу применяет сортировку и закрывает меню
 * (решение владельца 2026-09-03); на мобильной ширине вместо меню
 * остаётся шит ContactsSortSheet (макет 1539:85395). */
export type ContactsSortMenuProps = {
  readonly order: ContactSortOrder;
  readonly onOrderChange: (order: ContactSortOrder) => void;
  readonly children: ReactNode;
};

export function ContactsSortMenu({
  order,
  onOrderChange,
  children,
}: ContactsSortMenuProps): JSX.Element {
  return (
    <Menu>
      <MenuTrigger asChild>{children}</MenuTrigger>
      <MenuContent align="start" className="w-[246px] gap-0">
        <div className="flex flex-col gap-0.5">
          <SortMenuOption
            label="Имя от А до Я"
            selected={order === 'asc'}
            onSelect={() => onOrderChange('asc')}
          />
          <SortMenuOption
            label="Имя от Я до А"
            selected={order === 'desc'}
            onSelect={() => onOrderChange('desc')}
          />
        </div>
      </MenuContent>
    </Menu>
  );
}

/** Пункт меню (Figma 1186:44732 + Selection Button Checkbox, radius 8):
 * подпись 14/500 и квадрат выбора — синий с белой галкой у выбранного. */
function SortMenuOption({
  label,
  selected,
  onSelect,
}: {
  readonly label: string;
  readonly selected: boolean;
  readonly onSelect: () => void;
}): JSX.Element {
  return (
    <MenuItem
      onSelect={onSelect}
      icon={
        selected ? (
          <span className="flex h-5 w-5 items-center justify-center rounded-lg bg-primary text-white">
            <Check className="h-4 w-4" aria-hidden />
          </span>
        ) : (
          <span className="h-5 w-5 rounded-lg border-[1.5px] border-content-tertiary" aria-hidden />
        )
      }
    >
      {label}
    </MenuItem>
  );
}
