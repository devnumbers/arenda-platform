'use client';

import type { JSX } from 'react';
import { StatusGood } from '@/shared/assets/icons';
import { Icon } from '@/shared/ui/icon';
import { LinkButton } from '@/shared/ui/link-button';
import { ROUTES } from '@/shared/config/routes';
import { type OperationType } from '@/entities/operation/model/types';
import styles from './OperationSuccessScreen.module.css';

export type OperationSuccessScreenProps = {
  readonly type: OperationType;
  readonly propertyId?: string;
};

export function OperationSuccessScreen({
  type,
  propertyId,
}: OperationSuccessScreenProps): JSX.Element {
  const propertyHref = propertyId
    ? ROUTES.property(propertyId)
    : ROUTES.properties;

  return (
    <div className={styles.root}>
      <div className={styles.card}>
        <div className={styles.iconWrapper}>
          <Icon size="l">
            <StatusGood />
          </Icon>
        </div>
        <div className={styles.text}>
          <h2 className={styles.heading}>Операция создана</h2>
          <p className={styles.subtext}>
            {type === 'income'
              ? 'Доход добавлен и отобразится в списке финансов'
              : 'Расход добавлен и отобразится в списке финансов'}
          </p>
        </div>
        <div className={styles.actions}>
          <LinkButton
            href={ROUTES.finance}
            variant="secondary"
            size="large"
            fullWidth
          >
            К финансам
          </LinkButton>
          <LinkButton
            href={propertyHref}
            variant="primary"
            size="large"
            fullWidth
          >
            К объекту
          </LinkButton>
        </div>
      </div>
    </div>
  );
}
