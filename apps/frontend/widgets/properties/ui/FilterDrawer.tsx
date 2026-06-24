'use client';

import type { JSX, ReactNode } from 'react';
import { Drawer, DrawerBody, DrawerContent, DrawerFooter, DrawerHeader } from '@heroui/react/drawer';
import { Button } from '@/shared/ui/button';
import styles from './FilterDrawer.module.css';

export type FilterDrawerProps = {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly onApply: () => void;
  readonly children: ReactNode;
};

export function FilterDrawer({ isOpen, onClose, onApply, children }: FilterDrawerProps): JSX.Element {
  return (
    <Drawer isOpen={isOpen} onOpenChange={(open) => { if (!open) onClose(); }}>
      <DrawerContent placement="bottom">
        <DrawerHeader className={styles.header}>Фильтры</DrawerHeader>
        <DrawerBody className={styles.body}>{children}</DrawerBody>
        <DrawerFooter className={styles.footer}>
          <Button variant="primary" size="large" fullWidth onClick={onApply}>
            Применить
          </Button>
        </DrawerFooter>
      </DrawerContent>
    </Drawer>
  );
}
