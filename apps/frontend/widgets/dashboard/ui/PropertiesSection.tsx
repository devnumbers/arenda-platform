'use client';

import type { JSX } from 'react';
import NextLink from 'next/link';
import { Card } from '@heroui/react/card';
import { Skeleton } from '@heroui/react/skeleton';
import { Icon } from '@/shared/ui/icon';
import { Objects } from '@/shared/assets/icons';
import type { components } from '@/shared/api/generated';
import { EmptyState } from './EmptyState';
import styles from './PropertiesSection.module.css';

type PropertyResponse = components['schemas']['PropertyResponse'];

type PropertiesSectionProps = {
  readonly properties: PropertyResponse[] | undefined;
  readonly isLoading: boolean;
};

export function PropertiesSection({
  properties,
  isLoading,
}: PropertiesSectionProps): JSX.Element {
  const count = properties?.length ?? 0;

  if (isLoading) {
    return (
      <section className={styles.section}>
        <Skeleton className={styles.titleSkeleton} />
        <div className={styles.list}>
          <Skeleton className={styles.itemSkeleton} />
          <Skeleton className={styles.itemSkeleton} />
          <Skeleton className={styles.itemSkeleton} />
        </div>
      </section>
    );
  }

  if (!properties || properties.length === 0) {
    return (
      <section className={styles.section}>
        <h2 className={styles.title}>Объекты</h2>
        <EmptyState
          icon={<Objects />}
          entities="объектов"
          subtitle="Добавьте объект недвижимости"
          actionHref="/properties"
          actionText="Добавить объект"
        />
      </section>
    );
  }

  return (
    <section className={styles.section}>
      <div className={styles.header}>
        <h2 className={styles.title}>Объекты {count}</h2>
        <NextLink href="/properties" className={styles.link}>
          Все
        </NextLink>
      </div>
      <div className={styles.scroll}>
        {properties.map((property) => (
          <NextLink
            key={property.id}
            href={`/properties/${property.id}`}
            className={styles.cardLink}
          >
            <Card className={styles.card}>
              <div className={styles.placeholder} />
              <span className={styles.name}>{property.name}</span>
            </Card>
          </NextLink>
        ))}
        <NextLink href="/properties" className={styles.allCard}>
          <Icon size="m">
            <Objects />
          </Icon>
          <span>Все</span>
        </NextLink>
      </div>
    </section>
  );
}
