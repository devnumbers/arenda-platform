'use client';

import type { JSX } from 'react';
import { Cancel } from '@/shared/assets/icons';
import { IconButton } from '@/shared/ui/icon-button';
import styles from './TenantCreateHeader.module.css';

export type TenantCreateHeaderProps = {
  readonly onClose: () => void;
};

export function TenantCreateHeader({ onClose }: TenantCreateHeaderProps): JSX.Element {
  return (
    <header className={styles.root}>
      <IconButton
        variant="icon-black"
        size="small"
        icon={<Cancel />}
        aria-label="Закрыть"
        onClick={onClose}
        className={styles.iconButton}
      />
      <h1 className={styles.title}>Добавление арендатора</h1>
      <div className={styles.spacer} />
    </header>
  );
}
