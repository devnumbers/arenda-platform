'use client';

import type { JSX } from 'react';
import { IconLink } from '@/shared/ui/icon-link';
import { LinkButton } from '@/shared/ui/link-button';
import { ArrowLeft } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import styles from './TenantDetailHeader.module.css';

export type TenantDetailHeaderProps = {
  readonly title: string;
  readonly tenantId: string;
};

export function TenantDetailHeader({ title, tenantId }: TenantDetailHeaderProps): JSX.Element {
  return (
    <header className={styles.root}>
      <IconLink href={ROUTES.tenants} aria-label="Назад" icon={<ArrowLeft />} />
      <h1 className={styles.title}>{title}</h1>
      <LinkButton href={ROUTES.tenantEdit(tenantId)} variant="secondary" size="small">
        Редактировать
      </LinkButton>
    </header>
  );
}
