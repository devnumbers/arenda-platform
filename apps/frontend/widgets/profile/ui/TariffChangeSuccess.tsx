'use client';

import type { JSX } from 'react';
import { StatusGood } from '@/shared/assets/icons';
import { Icon } from '@/shared/ui/icon';
import { LinkButton } from '@/shared/ui/link-button';
import { useSubscription } from '@/features/billing/api/hooks';
import { formatDate } from '@/shared/lib/format-date';
import { ROUTES } from '@/shared/config/routes';
import styles from './TariffChangeSuccess.module.css';

export function TariffChangeSuccess(): JSX.Element {
  const { data: subscription, isPending } = useSubscription();

  const pending = subscription?.pendingTariff;
  const pendingChangeAt = subscription?.pendingChangeAt;

  return (
    <div className={styles.root}>
      <div className={styles.card}>
        <div className={styles.iconWrapper}>
          <Icon size="l">
            <StatusGood />
          </Icon>
        </div>

        <div className={styles.text}>
          <h2 className={styles.heading}>Тариф изменен</h2>
          {isPending ? (
            <p className={styles.subtext}>Загрузка сведений о подписке...</p>
          ) : (
            <p className={styles.subtext}>
              {pending && pendingChangeAt
                ? `Изменения вступят в силу ${formatDate(pendingChangeAt)}.`
                : 'Тариф успешно изменен.'}
            </p>
          )}
        </div>

        <LinkButton
          href={ROUTES.profileTariff}
          variant="primary"
          size="large"
          fullWidth
        >
          Вернуться к тарифу
        </LinkButton>
      </div>
    </div>
  );
}
