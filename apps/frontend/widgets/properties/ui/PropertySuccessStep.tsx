'use client';

import type { JSX } from 'react';
import { Button } from '@/shared/ui/button';
import { Icon } from '@/shared/ui/icon';
import { HomeAdd, Key } from '@/shared/assets/icons';
import styles from './PropertySuccessStep.module.css';

export type PropertySuccessStepProps = {
  onAddLater: () => void;
  onCreateLease: () => void;
};

export function PropertySuccessStep({
  onAddLater,
  onCreateLease,
}: PropertySuccessStepProps): JSX.Element {
  return (
    <div className={styles.root}>
      <div className={styles.card}>
        <div className={styles.iconWrapper}>
          <Icon size="l">
            <HomeAdd />
          </Icon>
        </div>

        <div className={styles.text}>
          <h2 className={styles.heading}>Объект создан</h2>
          <p className={styles.subtext}>
            Теперь вы можете создать и отслеживать аренду
          </p>
        </div>

        <div className={styles.actions}>
          <Button
            type="button"
            variant="secondary"
            size="large"
            fullWidth
            onClick={onAddLater}
          >
            Добавить позже
          </Button>
          <Button
            type="button"
            variant="primary"
            size="large"
            fullWidth
            leftIcon={
              <Icon size="m">
                <Key />
              </Icon>
            }
            onClick={onCreateLease}
          >
            Создать аренду
          </Button>
        </div>
      </div>
    </div>
  );
}
