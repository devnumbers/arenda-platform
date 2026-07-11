'use client';

import type { JSX } from 'react';
import { Tooltip } from '@heroui/react';
import { ROUTES } from '@/shared/config/routes';
import { EmptyState } from '@/shared/ui/empty-state';
import { HomeAdd } from '@/shared/assets/icons';
import { Icon } from '@/shared/ui/icon';
import styles from './PropertiesEmptyState.module.css';

export type PropertiesEmptyStateProps = {
  readonly canAdd?: boolean;
  readonly isLoading?: boolean;
};

export function PropertiesEmptyState({ canAdd, isLoading }: PropertiesEmptyStateProps): JSX.Element {
  if (isLoading) {
    return (
      <EmptyState
        imageSrc="/images/empty-logo.png"
        imageAlt="Логотип"
        title="Здесь будут отображаться ваши объекты"
        subtitle="Добавьте свою квартиру, студию, помещение или другой объект недвижимости"
        actionNode={
          <button
            type="button"
            disabled
            className={styles.actionDisabled}
            aria-label="Создать объект"
          >
            <Icon size="m">
              <HomeAdd />
            </Icon>
            Создать объект
          </button>
        }
      />
    );
  }

  if (canAdd === false) {
    return (
      <EmptyState
        imageSrc="/images/empty-logo.png"
        imageAlt="Логотип"
        title="Здесь будут отображаться ваши объекты"
        subtitle="Добавьте свою квартиру, студию, помещение или другой объект недвижимости"
        actionNode={
          <Tooltip>
            <Tooltip.Trigger>
              <button
                type="button"
                disabled
                className={styles.actionDisabled}
                aria-label="Создать объект (достигнут лимит)"
              >
                <Icon size="m">
                  <HomeAdd />
                </Icon>
                Создать объект
              </button>
            </Tooltip.Trigger>
            <Tooltip.Content>Достигнут лимит объектов по тарифу</Tooltip.Content>
          </Tooltip>
        }
      />
    );
  }

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
