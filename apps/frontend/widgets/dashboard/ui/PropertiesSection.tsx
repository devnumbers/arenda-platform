'use client';

import type {JSX} from 'react';
import {Skeleton} from '@heroui/react/skeleton';
import {Icon} from '@/shared/ui/icon';
import {Home, Objects} from '@/shared/assets/icons';
import {EmptyState} from '@/shared/ui/empty-state';
import type {Property} from '@/entities/property/model/types';
import {SectionHeader} from '@/shared/ui/section-header';
import {EntityCard} from './EntityCard';
import {IconActionCard} from './IconActionCard';
import styles from './PropertiesSection.module.css';

type PropertiesSectionProps = {
    readonly properties: Property[] | undefined;
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
                <Skeleton className={styles.titleSkeleton}/>
                <div className={styles.scroll}>
                    <Skeleton className={styles.itemSkeleton}/>
                    <Skeleton className={styles.itemSkeleton}/>
                    <Skeleton className={styles.itemSkeleton}/>
                </div>
            </section>
        );
    }

    if (!properties || properties.length === 0) {
        return (
            <section className={styles.section}>
                <SectionHeader title="Объекты" href="/properties"/>
                <EmptyState
                    icon={<Objects/>}
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
            <SectionHeader title="Объекты" count={count} href="/properties"/>
            <div className={styles.scroll}>
                {properties.map((property) => (
                    <EntityCard
                        key={property.id}
                        href={`/properties/${property.id}`}
                        title={property.name}
                        icon={
                            <Icon size="l">
                                <Home/>
                            </Icon>
                        }
                    />
                ))}
                <IconActionCard
                    href="/properties"
                    icon={
                        <Icon size="l">
                            <Objects/>
                        </Icon>
                    }
                    label="Все"
                    variant="outlined"
                    className={styles.allCard}
                />
            </div>
        </section>
    );
}
