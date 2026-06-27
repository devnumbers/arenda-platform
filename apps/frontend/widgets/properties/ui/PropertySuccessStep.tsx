'use client';

import type { JSX } from 'react';
import { Button } from '@/shared/ui/button';
import { Icon } from '@/shared/ui/icon';
import { Key } from '@/shared/assets/icons';
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
          <svg
            className={styles.illustration}
            viewBox="0 0 20 20"
            fill="none"
            xmlns="http://www.w3.org/2000/svg"
          >
            <path
              d="M3 8L10 3L17 8"
              stroke="currentColor"
              strokeWidth="1.5"
              strokeLinecap="round"
              strokeLinejoin="round"
            />
            <path
              d="M4.5 9.5V15.5C4.5 16 4.8 16.5 5.3 16.5H11"
              stroke="currentColor"
              strokeWidth="1.5"
              strokeLinecap="round"
              strokeLinejoin="round"
            />
            <circle
              cx="14.5"
              cy="13.5"
              r="3.5"
              stroke="currentColor"
              strokeWidth="1.5"
            />
            <path
              d="M14.5 11.5V15.5"
              stroke="currentColor"
              strokeWidth="1.5"
              strokeLinecap="round"
            />
            <path
              d="M12.5 13.5H16.5"
              stroke="currentColor"
              strokeWidth="1.5"
              strokeLinecap="round"
            />
          </svg>
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
