'use client';

import { cloneElement, isValidElement, type JSX, type ReactElement, type ReactNode } from 'react';
import Image from 'next/image';
import { LinkButton } from '@/shared/ui/link-button';
import styles from './EmptyState.module.css';

export type EmptyStateProps = {
  readonly icon?: ReactElement;
  readonly imageSrc?: string;
  readonly imageAlt?: string;
  readonly entities?: string;
  readonly subtitle: string;
  readonly actionHref: string;
  readonly actionText: string;
  readonly actionIcon?: ReactNode;
  readonly title?: string;
};

export function EmptyState({
  icon,
  imageSrc,
  imageAlt = '',
  entities,
  subtitle,
  actionHref,
  actionText,
  actionIcon,
  title,
}: EmptyStateProps): JSX.Element {
  const sizedIcon = isValidElement(icon)
    ? cloneElement(icon, { width: 96, height: 96 } as Record<string, unknown>)
    : icon;

  return (
    <div className={styles.root}>
      {imageSrc ? (
        <Image
          className={styles.image}
          src={imageSrc}
          alt={imageAlt}
          width={96}
          height={96}
          priority
        />
      ) : (
        <div className={styles.iconWrapper}>{sizedIcon}</div>
      )}
      <div className={styles.text}>
        <h3 className={styles.title}>{title ?? `Нет ${entities ?? ''}`}</h3>
        <p className={styles.subtitle}>{subtitle}</p>
      </div>
      <LinkButton
        href={actionHref}
        variant="primary"
        size="medium"
        leftIcon={actionIcon}
      >
        {actionText}
      </LinkButton>
    </div>
  );
}
