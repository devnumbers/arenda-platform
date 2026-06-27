'use client';

import type { JSX, ReactNode } from 'react';
import { IconLink } from '@/shared/ui/icon-link';
import { ArrowLeft } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import styles from './PropertyDetailHeader.module.css';

export type PropertyDetailHeaderProps = {
  readonly title?: string;
  readonly actions?: ReactNode;
};

export function PropertyDetailHeader({
  title = 'Мой объект',
  actions,
}: PropertyDetailHeaderProps): JSX.Element {
  return (
    <header className={styles.root}>
      <IconLink
        href={ROUTES.properties}
        aria-label="Назад"
        icon={<ArrowLeft />}
      />
      <h1 className={styles.title}>{title}</h1>
      {actions}
    </header>
  );
}
