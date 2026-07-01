import type { JSX, ReactNode } from 'react';
import { PageHeader } from '@/shared/ui/page-header';
import { ROUTES } from '@/shared/config/routes';

export interface TenantDetailHeaderProps {
  title: ReactNode;
  actions?: ReactNode;
}

export function TenantDetailHeader({ title, actions }: TenantDetailHeaderProps): JSX.Element {
  return <PageHeader title={title} backHref={ROUTES.tenants} actions={actions} />;
}
