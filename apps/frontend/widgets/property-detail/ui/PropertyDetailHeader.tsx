import type { JSX, ReactNode } from 'react';
import { PageHeader } from '@/shared/ui/page-header';
import { ROUTES } from '@/shared/config/routes';

export interface PropertyDetailHeaderProps {
  title?: string;
  actions?: ReactNode;
}

export function PropertyDetailHeader({ title = 'Мой объект', actions }: PropertyDetailHeaderProps): JSX.Element {
  return <PageHeader title={title} backHref={ROUTES.properties} actions={actions} />;
}
