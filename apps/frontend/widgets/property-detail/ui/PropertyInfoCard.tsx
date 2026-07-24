'use client';

import type {JSX} from 'react';
import {LinkButton} from '@/shared/ui/link-button';
import {ROUTES} from '@/shared/config/routes';
import {SectionHeader} from '@/widgets/dashboard/ui/SectionHeader';
import {PropertyDetailSection} from './PropertyDetailSection';
import styles from './PropertyInfoCard.module.css';

export type PropertyInfoCardProps = {
    readonly description: string | undefined;
    readonly propertyId: string;
    readonly isArchived?: boolean;
};

export function PropertyInfoCard({
                                     description,
                                     propertyId,
                                     isArchived = false,
                                 }: PropertyInfoCardProps): JSX.Element {
    return (
        <PropertyDetailSection>
            <SectionHeader title="Информация об объекте"/>

            {description ? (
                <p className={styles.description}>{description}</p>
            ) : (
                <LinkButton
                    href={ROUTES.propertyEdit(propertyId)}
                    variant="primary"
                    fullWidth
                    disabled={isArchived}
                    title={isArchived ? 'Объект в архиве' : undefined}
                >
                    Добавить описание
                </LinkButton>
            )}
        </PropertyDetailSection>
    );
}
