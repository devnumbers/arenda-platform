'use client';

import type { JSX } from 'react';
import { ROUTES } from '@/shared/config/routes';
import { EmptyState } from '@/shared/ui/empty-state';
import { HomeAdd } from '@/shared/assets/icons';

export function PropertiesEmptyState(): JSX.Element {
  return (
    <EmptyState
      icon={<HomeAdd />}
      title="Здесь будут отображаться ваши объекты"
      entities="объектов"
      subtitle="Добавьте свою квартиру, студию, помещение или другой объект недвижимости"
      actionHref={ROUTES.propertyNew}
      actionText="Создать объект"
    />
  );
}
