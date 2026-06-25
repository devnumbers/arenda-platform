'use client';

import type { JSX } from 'react';
import { ArrowLeft, Cancel } from '@/shared/assets/icons';
import { IconButton } from '@/shared/ui/icon-button';
import styles from './LeaseCreateHeader.module.css';

export type LeaseCreateHeaderProps = {
  step: 1 | 2;
  onBack: () => void;
  onCancel: () => void;
};

export function LeaseCreateHeader({ step, onBack, onCancel }: LeaseCreateHeaderProps): JSX.Element {
  return (
    <header className={styles.root}>
      <div className={styles.topRow}>
        <IconButton
          variant="icon-black"
          size="small"
          icon={<ArrowLeft />}
          aria-label="Назад"
          onClick={onBack}
          className={styles.iconButton}
        />
        <h1 className={styles.title}>Создание аренды</h1>
        <IconButton
          variant="icon-black"
          size="small"
          icon={<Cancel />}
          aria-label="Отменить"
          onClick={onCancel}
          className={styles.iconButton}
        />
      </div>
      <div className={styles.progressRow}>
        <span className={styles.badge}>{step} из 2</span>
        <div className={styles.progressBar} role="progressbar" aria-label={`Шаг ${step} из 2`} aria-valuenow={step} aria-valuemin={1} aria-valuemax={2}>
          <div className={`${styles.segment} ${step >= 1 ? styles.active : ''}`} />
          <div className={`${styles.segment} ${step >= 2 ? styles.active : ''}`} />
        </div>
      </div>
    </header>
  );
}
