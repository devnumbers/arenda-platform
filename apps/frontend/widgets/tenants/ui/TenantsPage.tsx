'use client';

import { useMemo, type JSX } from 'react';
import { useTenantContacts } from '@/features/tenant-contacts/api';
import { PageHeader } from '@/shared/ui/page-header';
import { LinkButton } from '@/shared/ui/link-button';
import { ROUTES } from '@/shared/config/routes';
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
      <div className={styles.content}>
        <PageHeader
          title="Арендаторы"
          backHref={ROUTES.dashboard}
          actions={
            <LinkButton href={ROUTES.tenantNew} variant="primary" size="small">
              Добавить
            </LinkButton>
          }
        />

        <TenantSection title="Текущие арендаторы" tenants={active} />
        <TenantSection title="Прошлые арендаторы" tenants={past} />
      </div>
    </div>
  );
}
