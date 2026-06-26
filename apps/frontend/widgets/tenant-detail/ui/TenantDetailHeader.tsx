'use client';

import type { JSX } from 'react';
import { IconLink } from '@/shared/ui/icon-link';
import { Button } from '@/shared/ui/button';
import { ArrowLeft } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import styles from './TenantDetailHeader.module.css';

export type TenantDetailHeaderProps = {
  readonly title: string;
};

export function TenantDetailHeader({ title }: TenantDetailHeaderProps): JSX.Element {
  return (
    <header className={styles.root}>
      <IconLink href={ROUTES.tenants} aria-label="Назад" icon={<ArrowLeft />} />
      <h1 className={styles.title}>{title}</h1>
      <Button
        type="button"
        variant="secondary"
        size="small"
        disabled
        onClick={() => {
          // TODO: navigate to tenant edit page when implemented
        }}
      >
        Редактировать
      </Button>
    </header>
  );
}
