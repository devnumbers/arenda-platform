'use client';

import type { JSX, ReactNode } from 'react';
import { IconLink } from '@/shared/ui/icon-link';
import { ArrowLeft } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import styles from './TenantDetailHeader.module.css';

export type TenantDetailHeaderProps = {
  readonly title: string;
  readonly actions?: ReactNode;
};

export function TenantDetailHeader({ title, actions }: TenantDetailHeaderProps): JSX.Element {
  return (
    <header className={styles.root}>
      <IconLink href={ROUTES.tenants} aria-label="Назад" icon={<ArrowLeft />} />
      <h1 className={styles.title}>{title}</h1>
      {actions}
    </header>
  );
}
