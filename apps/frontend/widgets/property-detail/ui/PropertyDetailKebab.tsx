import type { JSX } from 'react';
import type { PropertyStatus } from '@/entities/property';
import { Archive, ChangeVertical, Info, Kebab, Team } from '@/shared/assets/icons';
import { Menu, MenuContent, MenuItem, MenuTrigger, IconButton } from '@/shared/ui/design';
import {
  buildPropertyKebabItems,
  type PropertyDetailActionKey,
} from '../lib/property-detail-status';

/** Иконки пунктов кебаба; мутации статуса недоступны смотрящему и в меню
 * не попадают (фильтрует buildPropertyKebabItems). */
const KEBAB_ICONS: Partial<Record<PropertyDetailActionKey, JSX.Element>> = {
  about: <Info className="h-6 w-6" />,
  'change-status': <ChangeVertical className="h-6 w-6" />,
  unarchive: <Archive className="h-6 w-6" />,
  access: <Team className="h-6 w-6" />,
};

export type PropertyDetailKebabProps = {
  readonly status: PropertyStatus;
  /** Ролевой мутационный доступ (смотрящему — только чтение и доступ). */
  readonly canMutate: boolean;
  readonly onAction: (key: PropertyDetailActionKey) => void;
};

/** Кебаб шапки детали (Figma 1186:44996 — активный: об объекте, изменить
 * статус, совместный доступ; 1581:52407 — архивный: об объекте, вернуть
 * из архива, совместный доступ). Канон Menu (Radix dropdown). */
export function PropertyDetailKebab({
  status,
  canMutate,
  onAction,
}: PropertyDetailKebabProps): JSX.Element {
  return (
    <Menu>
      <MenuTrigger asChild>
        <IconButton icon={<Kebab className="h-6 w-6" />} label="Действия с объектом" />
      </MenuTrigger>
      <MenuContent>
        {buildPropertyKebabItems(status, canMutate).map((item) => (
          <MenuItem
            key={item.key}
            icon={KEBAB_ICONS[item.key]}
            onSelect={() => onAction(item.key)}
          >
            {item.label}
          </MenuItem>
        ))}
      </MenuContent>
    </Menu>
  );
}
