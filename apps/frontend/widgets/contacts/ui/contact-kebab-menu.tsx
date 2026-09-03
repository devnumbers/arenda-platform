'use client';

import type { JSX } from 'react';
import * as DropdownMenu from '@radix-ui/react-dropdown-menu';
import { Edit, Kebab, TrashBin } from '@/shared/assets/icons';
import { IconButton } from '@/shared/ui/design';

/**
 * Кебаб-меню карточки контакта (#510, макет 1424-54725): тап по ⋮ в
 * шапке открывает привязанное к кнопке меню — белая карточка со скруглением
 * и тенью, пункты «Изменить» (Icon/R/Edit) и «Удалить» (Icon/R/TrashBin).
 * Примитив — Radix DropdownMenu (ADR 0050): клавиатура, фокус и закрытие
 * по клику вне/Escape — из коробки. Первый примитив DropdownMenu в
 * дизайн-контуре — при втором потребителе переезжает в shared/ui/design.
 */
export function ContactKebabMenu({
  onEdit,
  onDelete,
}: {
  readonly onEdit: () => void;
  readonly onDelete: () => void;
}): JSX.Element {
  return (
    <DropdownMenu.Root>
      <DropdownMenu.Trigger asChild>
        <IconButton icon={<Kebab className="h-6 w-6" />} label="Меню контакта" />
      </DropdownMenu.Trigger>
      <DropdownMenu.Portal>
        <DropdownMenu.Content
          align="end"
          sideOffset={4}
          collisionPadding={24}
          className="z-50 min-w-[250px] rounded-3xl bg-white p-2 shadow-[0_8px_32px_rgba(23,26,28,0.16)] outline-none"
        >
          <MenuItem onSelect={onEdit} icon={<Edit className="h-6 w-6" />} label="Изменить" />
          <MenuItem onSelect={onDelete} icon={<TrashBin className="h-[18px] w-[17px]" />} label="Удалить" />
        </DropdownMenu.Content>
      </DropdownMenu.Portal>
    </DropdownMenu.Root>
  );
}

/** Пункт меню (макет 1424-54725): иконка 24 + заголовок 16/500, строка 48. */
function MenuItem({
  onSelect,
  icon,
  label,
}: {
  readonly onSelect: () => void;
  readonly icon: JSX.Element;
  readonly label: string;
}): JSX.Element {
  return (
    <DropdownMenu.Item
      onSelect={onSelect}
      className="flex h-12 cursor-pointer select-none items-center gap-3 rounded-2xl px-3 text-base font-medium leading-[18px] text-content outline-none transition-colors data-[highlighted]:bg-surface-muted"
    >
      <span aria-hidden className="flex h-6 w-6 shrink-0 items-center justify-center">
        {icon}
      </span>
      {label}
    </DropdownMenu.Item>
  );
}
