'use client';

import type { JSX } from 'react';
import { ROUTES } from '@/shared/config/routes';
import { EmptyState } from '@/shared/ui/empty-state';
import { HomeAdd } from '@/shared/assets/icons';
import { Icon } from '@/shared/ui/icon';

export function PropertiesEmptyState(): JSX.Element {
  return (
    <EmptyState
      imageSrc="/images/empty-logo.png"
      imageAlt="Логотип"
      title="Здесь будут отображаться ваши объекты"
      subtitle="Добавьте свою квартиру, студию, помещение или другой объект недвижимости"
      actionHref={ROUTES.propertyNew}
      actionText="Создать объект"
      actionIcon={
        <Icon size="m">
          <HomeAdd />
        </Icon>
      }
    />
  );
}
