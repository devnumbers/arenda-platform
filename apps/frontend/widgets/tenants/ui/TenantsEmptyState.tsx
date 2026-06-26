'use client';

import type { JSX } from 'react';
import { Icon } from '@/shared/ui/icon';
import { EmptyState } from '@/shared/ui/empty-state';
import { Arendators } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';

export function TenantsEmptyState(): JSX.Element {
  return (
    <EmptyState
      icon={
        <Icon size="l">
          <Arendators />
        </Icon>
      }
      entities="арендаторов"
      subtitle="Добавьте первого арендатора"
      actionHref={ROUTES.tenantNew}
      actionText="Добавить арендатора"
    />
  );
}
