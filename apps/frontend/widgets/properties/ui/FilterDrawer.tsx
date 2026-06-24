'use client';

import type { JSX, ReactNode } from 'react';
import {
  Drawer,
  DrawerBackdrop,
  DrawerBody,
  DrawerCloseTrigger,
  DrawerContent,
  DrawerDialog,
  DrawerFooter,
  DrawerHeader,
  DrawerHeading,
} from '@heroui/react/drawer';
import { Button as HeroButton } from '@heroui/react/button';
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
      <DrawerBackdrop />
      <DrawerContent placement="bottom">
        <DrawerDialog>
          <DrawerCloseTrigger aria-label="Закрыть" />
          <DrawerHeader className={styles.header}>
            <DrawerHeading>Фильтры</DrawerHeading>
          </DrawerHeader>
          <DrawerBody className={styles.body}>{children}</DrawerBody>
          <DrawerFooter className={styles.footer}>
            <HeroButton variant="primary" size="lg" fullWidth onClick={onApply}>
              Применить
            </HeroButton>
          </DrawerFooter>
        </DrawerDialog>
      </DrawerContent>
    </Drawer>
  );
}
