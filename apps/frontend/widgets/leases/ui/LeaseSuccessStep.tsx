'use client';

import type { JSX } from 'react';
import { Button } from '@/shared/ui/button';
import { Icon } from '@/shared/ui/icon';
import { BoldUsers, Key } from '@/shared/assets/icons';
import styles from './LeaseSuccessStep.module.css';

export type LeaseSuccessStepProps = {
  readonly onAddLater: () => void;
  readonly onAddTenant: () => void;
};

export function LeaseSuccessStep({
  onAddLater,
  onAddTenant,
}: LeaseSuccessStepProps): JSX.Element {
  return (
    <div className={styles.root}>
      <div className={styles.card}>
        <div className={styles.iconWrapper}>
          <Icon size="m">
            <Key />
          </Icon>
        </div>

        <div className={styles.text}>
          <h2 className={styles.heading}>Аренда создана</h2>
          <p className={styles.subtext}>
            Добавьте арендатора и его контакты, чтобы все данные были в одном месте
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
                <BoldUsers />
              </Icon>
            }
            onClick={onAddTenant}
          >
            Добавить арендатора
          </Button>
        </div>
      </div>
    </div>
  );
}
