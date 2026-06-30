'use client';

import type { JSX, ReactNode } from 'react';
import { Icon } from '@/shared/ui/icon';
import { BoldWallet } from '@/shared/assets/icons';
import { LinkButton } from '@/shared/ui/link-button';
import styles from './FinanceEmptyState.module.css';

export type FinanceEmptyStateProps = {
  readonly title?: string;
  readonly subtitle?: string;
  readonly actionHref?: string;
  readonly actionText?: string;
};

export function FinanceEmptyState({
  title = 'Нет данных',
  subtitle = 'Здесь будут отображаться финансовые операции',
  actionHref,
  actionText,
}: FinanceEmptyStateProps): JSX.Element {
  const actionContent: ReactNode | null =
    actionHref && actionText ? (
      <LinkButton href={actionHref} variant="primary" size="medium">
        {actionText}
      </LinkButton>
    ) : null;

  return (
    <div className={styles.root}>
      <div className={styles.iconWrapper}>
        <Icon size="l">
          <BoldWallet />
        </Icon>
      </div>
      <div className={styles.text}>
        <h3 className={styles.title}>{title}</h3>
        {subtitle && <p className={styles.subtitle}>{subtitle}</p>}
      </div>
      {actionContent}
    </div>
  );
}
