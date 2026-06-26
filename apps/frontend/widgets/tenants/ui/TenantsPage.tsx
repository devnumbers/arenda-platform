'use client';

import { useMemo, type JSX } from 'react';
import { useTenantContacts } from '@/features/tenant-contacts/api';
import { LinkButton } from '@/shared/ui/link-button';
import { IconLink } from '@/shared/ui/icon-link';
import { ROUTES } from '@/shared/config/routes';
import { ArrowLeft } from '@/shared/assets/icons';
import { TenantSection } from './TenantSection';
import { TenantsLoading } from './TenantsLoading';
import { TenantsError } from './TenantsError';
import { TenantsEmptyState } from './TenantsEmptyState';
import styles from './TenantsPage.module.css';

export function TenantsPage(): JSX.Element {
  const query = useTenantContacts();

  const { active, past } = useMemo(() => {
    const tenants = query.data ?? [];
    const activeContacts = tenants.filter((t) => t.isActive);
    const pastContacts = tenants.filter((t) => !t.isActive);
    return { active: activeContacts, past: pastContacts };
  }, [query.data]);

  if (query.isPending) {
    return <TenantsLoading />;
  }

  if (query.isError) {
    return <TenantsError onRetry={() => query.refetch()} isLoading={query.isFetching} />;
  }

  if ((query.data?.length ?? 0) === 0) {
    return <TenantsEmptyState />;
  }

  return (
    <div className={styles.root}>
      <header className={styles.header}>
        <IconLink href={ROUTES.dashboard} aria-label="Назад" icon={<ArrowLeft />} />
        <h1 className={styles.title}>Арендаторы</h1>
        <LinkButton href={ROUTES.tenantNew} variant="secondary" size="small">
          Добавить
        </LinkButton>
      </header>

      <TenantSection title="Текущие арендаторы" tenants={active} />
      <TenantSection title="Прошлые арендаторы" tenants={past} />
    </div>
  );
}
