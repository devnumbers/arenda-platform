import type { JSX } from 'react';
import { Archive, ChangeVertical, Info, Kebab, Team } from '@/shared/assets/icons';
import { Menu, MenuContent, MenuItem, MenuTrigger, IconButton } from '@/shared/ui/design';
import {
  buildPropertyKebabItems,
  type PropertyDetailActionKey,
} from '../lib/property-detail-status';

const KEBAB_ITEMS: Record<
  PropertyDetailActionKey,
  { readonly label: string; readonly icon: JSX.Element } | null
> = {
  about: { label: 'Об объекте', icon: <Info className="h-6 w-6" /> },
  'change-status': { label: 'Изменить статус', icon: <ChangeVertical className="h-6 w-6" /> },
  unarchive: { label: 'Вернуть из архива', icon: <Archive className="h-6 w-6" /> },
  access: { label: 'Совместный доступ', icon: <Team className="h-6 w-6" /> },
  edit: null,
  pin: null,
  unpin: null,
  'start-rental': null,
  'complete-rental': null,
  'start-maintenance': null,
  'finish-maintenance': null,
  archive: null,
  delete: null,
};

export type PropertyDetailKebabProps = {
  readonly status: 'active' | 'maintenance' | 'archived';
  readonly disabled?: boolean;
  readonly onAction: (key: PropertyDetailActionKey) => void;
};

/** Кебаб шапки детали (Figma 1186:44996 — активный: об объекте, изменить
 * статус, совместный доступ; 1581:52407 — архивный: об объекте, вернуть
 * из архива, совместный доступ). Канон Menu (Radix dropdown). */
export function PropertyDetailKebab({
  status,
  disabled = false,
  onAction,
}: PropertyDetailKebabProps): JSX.Element {
  return (
    <Menu>
      <MenuTrigger asChild>
        <IconButton icon={<Kebab className="h-6 w-6" />} label="Действия с объектом" disabled={disabled} />
      </MenuTrigger>
      <MenuContent>
        {buildPropertyKebabItems(status).map(({ key }) => {
          const item = KEBAB_ITEMS[key];
          if (item === null) return null;
          return (
            <MenuItem key={key} icon={item.icon} onSelect={() => onAction(key)}>
              {item.label}
            </MenuItem>
          );
        })}
      </MenuContent>
    </Menu>
  );
}
