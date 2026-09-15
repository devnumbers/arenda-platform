import type { JSX } from 'react';
import {
  Archive,
  Edit,
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
    case 'about':
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

/** Шит смены статуса (Figma 1554:100371 — без аренды, 1581:53679 — на
 * ремонте): строки-пилюли 56 с иконкой и подписью, под ними «Отменить».
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
              className="flex h-14 w-full cursor-pointer items-center justify-center gap-3 rounded-button bg-surface-muted text-left outline-none transition-colors focus-visible:ring-4 focus-visible:ring-primary hover:bg-surface-muted-hover active:bg-surface-muted-hover"
            >
              <span className="flex h-6 w-6 shrink-0 items-center justify-center" aria-hidden>
                {propertyActionIcon(item.key)}
              </span>
              <span className="text-base font-medium text-content">{item.label}</span>
            </button>
          ))}
        </div>
        <div className="mt-2 flex justify-center">
          <button
            type="button"
            onClick={() => onOpenChange(false)}
            className="flex h-11 cursor-pointer items-center rounded-pill px-5 text-base font-medium text-content outline-none transition-colors focus-visible:ring-2 focus-visible:ring-primary hover:bg-surface-muted active:bg-surface-muted"
          >
            Отменить
          </button>
        </div>
      </ModalContent>
    </Modal>
  );
}
