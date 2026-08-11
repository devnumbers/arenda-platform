import type {JSX} from 'react';
import type {PropertyType, PropertyAttributes} from '@/entities/property/model/types';
import {formatAttributesForCardGrouped} from '@/features/property-attributes';
import {ROUTES} from '@/shared/config/routes';
import {LinkButton} from '@/shared/ui/link-button';
import {SectionHeader} from '@/widgets/dashboard/ui/SectionHeader';
import {PropertyDetailSection} from './PropertyDetailSection';
import styles from './PropertyAttributesSection.module.css';

export type PropertyAttributesSectionProps = {
    readonly type: PropertyType;
    readonly attributes: PropertyAttributes;
    readonly propertyId: string;
    readonly isArchived?: boolean;
};

export function PropertyAttributesSection({
    type,
    attributes,
    propertyId,
    isArchived = false,
}: PropertyAttributesSectionProps): JSX.Element {
    const groups = formatAttributesForCardGrouped(type, attributes);

    if (groups.length === 0) {
        return (
            <PropertyDetailSection>
                <SectionHeader title="Характеристики"/>
                <LinkButton
                    href={ROUTES.propertyEdit(propertyId)}
                    variant="primary"
                    fullWidth
                    disabled={isArchived}
                    title={isArchived ? 'Объект в архиве' : undefined}
                >
                    Добавить характеристики
                </LinkButton>
            </PropertyDetailSection>
        );
    }

    return (
        <PropertyDetailSection>
            <SectionHeader title="Характеристики"/>
            <div className={styles.card}>
                {groups.map((group) => (
                    <div key={group.group ?? 'root'} className={styles.group}>
                        {group.label !== null && (
                            <h3 className={styles.groupTitle}>{group.label}</h3>
                        )}
                        <dl className={styles.list}>
                            {group.items.map((item) => (
                                <div key={item.label} className={styles.row}>
                                    <dt className={styles.label}>{item.label}</dt>
                                    <dd className={styles.value}>{item.value}</dd>
                                </div>
                            ))}
                        </dl>
                    </div>
                ))}
            </div>
        </PropertyDetailSection>
    );
}
