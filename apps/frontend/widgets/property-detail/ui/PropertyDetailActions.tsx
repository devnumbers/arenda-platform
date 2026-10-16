import type { JSX } from 'react';
import {
  Archive,
  Edit,
  Exit,
  Info,
  Key,
  PaintBrush,
  Pin,
  PinOff,
  Team,
  TrashBin,
} from '@/shared/assets/icons';
import { Modal, ModalContent } from '@/shared/ui/design';
import type { PropertyDetailActionKey } from '../lib/property-detail-status';

/** Иконки действий (канон §10). «Ремонт» — канонный PaintBrush: глифа
 * молотка в R-стиле в каноне нет (BoldHammer жирнее макетного) — сверить
 * на приёмке #588, при расхождении экспортировать ноду из Figma. */
export function propertyActionIcon(key: PropertyDetailActionKey): JSX.Element | null {
  const iconClass = 'h-6 w-6';
  switch (key) {
    case 'edit':
      return <Edit className={iconClass} />;
    case 'pin':
      return <Pin className={iconClass} />;
    case 'unpin':
      return <PinOff className={iconClass} />;
    case 'start-rental':
    case 'complete-rental':
      return <Key className={iconClass} />;
    case 'delete-rental':
      return <TrashBin className={iconClass} />;
    case 'start-maintenance':
    case 'finish-maintenance':
      return <PaintBrush className={iconClass} />;
    case 'archive':
    case 'unarchive':
      return <Archive className={iconClass} />;
    case 'access':
      return <Team className={iconClass} />;
    case 'delete':
      return <TrashBin className={iconClass} />;
    case 'leave':
      return <Exit className={iconClass} />;
    case 'about':
      return <Info className={iconClass} />;
    case 'change-status':
      return null;
  }
}

type PropertyManageSectionProps = {
  readonly items: ReadonlyArray<{
    readonly key: PropertyDetailActionKey;
    readonly label: string;
    readonly danger: boolean;
  }>;
  readonly onAction: (key: PropertyDetailActionKey) => void;
};

/** Секция «Управление» (Figma 1554:98469): строки действий 52 на серой
 * карточке — иконка в слоте 44 и подпись 16/18; деструктивное — красным.
 * Заголовок без шеврона: это действия, а не переход. Нижний паддинг 12 —
 * нижний отступ карточки по макету. */
export function PropertyManageSection({
  items,
  onAction,
}: PropertyManageSectionProps): JSX.Element {
  return (
    <div className="flex flex-col px-3 pb-3" data-testid="property-manage-list">
      {items.map((item) => (
        <button
          key={item.key}
          type="button"
          onClick={() => onAction(item.key)}
          className={`flex h-[52px] w-full cursor-pointer items-center rounded-button px-1 text-left outline-none transition-colors focus-visible:ring-2 focus-visible:ring-primary hover:bg-surface-muted-hover active:bg-surface-muted-hover ${
            item.danger ? 'text-danger' : 'text-content'
          }`}
        >
          <span className="flex h-11 w-11 shrink-0 items-center justify-center" aria-hidden>
            {propertyActionIcon(item.key)}
          </span>
          <span className="pl-1 text-base font-medium">{item.label}</span>
        </button>
      ))}
    </div>
  );
}

type PropertyStatusSheetProps = {
  readonly open: boolean;
  readonly onOpenChange: (open: boolean) => void;
  readonly items: ReadonlyArray<{
    readonly key: PropertyDetailActionKey;
    readonly label: string;
  }>;
  readonly onAction: (key: PropertyDetailActionKey) => void;
};

/** Общая форма ряда шита — геометрия контейнера и фокус-ринг; фон и
 * отклик у рядов свои (макеты 1554-100758 / 1581-53679). */
const sheetRowClass =
  'flex h-14 w-full cursor-pointer items-center justify-center rounded-button outline-none transition-colors focus-visible:ring-4 focus-visible:ring-primary';

/** Пункты статусов — серые контейнеры (var(--light-gray)). */
const sheetItemClass = `${sheetRowClass} bg-surface-muted hover:bg-surface-muted-hover active:bg-surface-muted-hover`;

/** Шит смены статуса (Figma 1554:100371/100758 — без аренды,
 * 1581:53679 — на ремонте): строки-контейнеры 56 с иконкой и подписью,
 * под ними «Отменить» — тот же контейнер, но белый: в макетах её фон
 * var(--white) при серых пунктах (решение владельца 07.10, #1241),
 * отклик — по канону Button secondary (решение владельца 08.10).
 * Канон Modal — на ≥768 карточка. Заголовок sr-only: a11y-имя диалога. */
export function PropertyStatusSheet({
  open,
  onOpenChange,
  items,
  onAction,
}: PropertyStatusSheetProps): JSX.Element {
  return (
    <Modal open={open} onOpenChange={onOpenChange}>
      <ModalContent title="Изменить статус" titleSrOnly>
        <div className="flex flex-col gap-2" role="menu" aria-label="Изменить статус">
          {items.map((item) => (
            <button
              key={item.key}
              type="button"
              role="menuitem"
              onClick={() => {
                onOpenChange(false);
                onAction(item.key);
              }}
              className={`${sheetItemClass} gap-3 text-left`}
            >
              <span className="flex h-6 w-6 shrink-0 items-center justify-center" aria-hidden>
                {propertyActionIcon(item.key)}
              </span>
              <span className="text-base font-medium text-content">{item.label}</span>
            </button>
          ))}
        </div>
        <button
          type="button"
          onClick={() => onOpenChange(false)}
          className={`${sheetRowClass} mt-2 bg-surface text-base font-medium text-content hover:bg-surface-muted-hover active:bg-surface-muted-active`}
        >
          Отменить
        </button>
      </ModalContent>
    </Modal>
  );
}
