'use client';

import type {JSX} from 'react';
import NextLink from 'next/link';
import {ROUTES} from '@/shared/config/routes';
import {getDisplayStatus} from '@/features/properties';
import {AccessRoleBadge} from '@/entities/access';
import type {PropertyWithLease} from '../lib/use-property-list-data';
import {PropertyStatusBadge} from '@/features/properties';
import {PropertyThumbnail} from '@/entities/property';
import styles from './PropertyCard.module.css';

export type PropertyCardProps = {
    readonly property: PropertyWithLease;
};

export function PropertyCard({property}: PropertyCardProps): JSX.Element {
    const displayStatus = getDisplayStatus(property.status);

    return (
        <article className={styles.root}>
            <NextLink
                href={ROUTES.property(property.id)}
                className={styles.cardLink}
                aria-label={`Открыть объект ${property.name}`}
            />
            <div className={styles.header}>
                <div className={styles.meta}>
                    <h3 className={styles.title}>{property.name}</h3>
                    {displayStatus && <PropertyStatusBadge status={displayStatus}/>}
                    {property.access && property.access.role !== 'owner' && (
                        <AccessRoleBadge role={property.access.role}/>
                    )}
                </div>
                <PropertyThumbnail size="small"/>
            </div>

        </article>
    );
}
