'use client';

import type { JSX } from 'react';
import { Edit, Kebab, TrashBin } from '@/shared/assets/icons';
import { IconButton, Menu, MenuContent, MenuItem, MenuTrigger } from '@/shared/ui/design';

/**
 * Кебаб-меню карточки контакта (карта #510 → перекомпоновка #1333; решение
 * владельца 18.10): тап по ⋮ в шапке — «Изменить контакт» и danger-пункт
 * «Удалить» (канон опасных пунктов — «Отозвать доступ» экрана участника).
 * Правка — единственным входом через меню: прежний тап по карточке
 * «Информация» больше не правит. Примитив — канон Menu дизайн-слоя
 * (shared/ui/design/menu.tsx, как на деталке объекта, участнике и истории
 * платежа): клавиатура, фокус и закрытие по клику вне/Escape — из Radix,
 * стили и размеры иконок — из дизайн-слоя. collisionPadding 24 — прежний
 * отступ от краёв вьюпорта (кебаб у правого верхнего угла экрана).
 */
export function ContactKebabMenu({
  onEdit,
  onDelete,
}: {
  readonly onEdit: () => void;
  readonly onDelete: () => void;
}): JSX.Element {
  return (
    <Menu>
      <MenuTrigger asChild>
        <IconButton icon={<Kebab className="h-6 w-6" />} label="Меню контакта" />
      </MenuTrigger>
      <MenuContent collisionPadding={24}>
        <MenuItem icon={<Edit className="h-6 w-6" />} onSelect={onEdit}>
          Изменить контакт
        </MenuItem>
        <MenuItem
          icon={<TrashBin className="h-6 w-6 text-error" />}
          className="text-error"
          onSelect={onDelete}
        >
          Удалить
        </MenuItem>
      </MenuContent>
    </Menu>
  );
}
