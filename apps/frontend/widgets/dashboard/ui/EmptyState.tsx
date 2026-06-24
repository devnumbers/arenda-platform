'use client';

import { cloneElement, isValidElement, type JSX, type ReactElement } from 'react';
import { EmptyState as HeroEmptyState } from '@heroui/react/empty-state';
import { LinkButton } from '@/shared/ui/link-button';
import styles from './EmptyState.module.css';

export type EmptyStateProps = {
  readonly icon: ReactElement;
  readonly entities: string;
  readonly subtitle: string;
  readonly actionHref: string;
  readonly actionText: string;
};

export function EmptyState({
  icon,
  entities,
  subtitle,
  actionHref,
  actionText,
}: EmptyStateProps): JSX.Element {
  const sizedIcon = isValidElement(icon)
    ? cloneElement(icon, { width: 48, height: 48 } as Record<string, unknown>)
    : icon;

  return (
    <HeroEmptyState className={styles.root}>
      <div className={styles.iconWrapper}>{sizedIcon}</div>
      <h3 className={styles.title}>Нет {entities}</h3>
      <p className={styles.subtitle}>{subtitle}</p>
      <LinkButton
        href={actionHref}
        variant="primary"
        size="medium"
        fullWidth
      >
        {actionText}
      </LinkButton>
    </HeroEmptyState>
  );
}
