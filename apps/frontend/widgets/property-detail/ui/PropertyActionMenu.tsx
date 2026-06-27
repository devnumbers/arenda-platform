'use client';

import { useState, type JSX } from 'react';
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@heroui/react';
import { IconButton } from '@/shared/ui/icon-button';
import { Menu } from '@/shared/assets/icons';
import styles from './PropertyActionMenu.module.css';

export type PropertyActionMenuProps = {
  readonly onEdit: () => void;
  readonly onMaintenance: () => void;
  readonly onEndLease: () => void;
  readonly onArchive: () => void;
};

export function PropertyActionMenu({
  onEdit,
  onMaintenance,
  onEndLease,
  onArchive,
}: PropertyActionMenuProps): JSX.Element {
  const [isOpen, setIsOpen] = useState(false);

  const handleAction = (action: () => void) => {
    action();
    setIsOpen(false);
  };

  return (
    <Popover isOpen={isOpen} onOpenChange={setIsOpen}>
      <PopoverTrigger>
        <IconButton aria-label="Действия" icon={<Menu />} />
      </PopoverTrigger>
      <PopoverContent
        placement="bottom end"
        offset={8}
        className={styles.menu}
      >
        <button
          type="button"
          className={styles.item}
          onClick={() => handleAction(onEdit)}
        >
          Редактировать объект
        </button>
        <button
          type="button"
          className={styles.item}
          onClick={() => handleAction(onMaintenance)}
        >
          На ремонт
        </button>
        <button
          type="button"
          className={styles.item}
          onClick={() => handleAction(onEndLease)}
        >
          Завершить аренду
        </button>
        <button
          type="button"
          className={styles.item}
          onClick={() => handleAction(onArchive)}
        >
          Перевести в архив
        </button>
      </PopoverContent>
    </Popover>
  );
}
