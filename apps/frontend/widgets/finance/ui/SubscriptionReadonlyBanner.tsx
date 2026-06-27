'use client';

import type { JSX } from 'react';
import { LinkButton } from '@/shared/ui/link-button';
import { ROUTES } from '@/shared/config/routes';
import { useSubscription } from '@/features/subscription/api/hooks';
import { isSubscriptionReadonly } from '@/features/subscription/lib/is-subscription-readonly';
import styles from './SubscriptionReadonlyBanner.module.css';

export function SubscriptionReadonlyBanner(): JSX.Element | null {
  const { data: subscription } = useSubscription();

  if (!isSubscriptionReadonly(subscription)) {
    return null;
  }

  return (
    <div className={styles.root} role="alert">
      <p className={styles.text}>
        Подписка неактивна. Редактирование финансов временно недоступно.
      </p>
      <LinkButton href={ROUTES.profileTariff} variant="primary" size="small">
        Перейти к тарифам
      </LinkButton>
    </div>
  );
}
