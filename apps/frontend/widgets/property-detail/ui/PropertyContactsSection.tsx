'use client';

import type {JSX} from 'react';
import {ROUTES} from '@/shared/config/routes';
import {LinkButton} from '@/shared/ui/link-button';
import {SectionHeader} from '@/widgets/dashboard/ui/SectionHeader';
import {usePropertyContacts} from '@/features/property-contacts/api';
import {PropertyDetailSection} from './PropertyDetailSection';
import styles from './PropertyContactsSection.module.css';

export type PropertyContactsSectionProps = {
    readonly propertyId: string;
    readonly isArchived?: boolean;
};

export function PropertyContactsSection({
                                           propertyId,
                                           isArchived = false,
                                       }: PropertyContactsSectionProps): JSX.Element {
    const {data: contacts, isPending, isError} = usePropertyContacts(propertyId);

    return (
        <PropertyDetailSection>
            <SectionHeader title="Контакты" count={contacts?.length}/>

            {isPending && (
                <p className={styles.errorText}>Загрузка…</p>
            )}

            {isError && (
                <p className={styles.errorText}>Не удалось загрузить контакты</p>
            )}

            {!isPending && !isError && (!contacts || contacts.length === 0) && (
                <LinkButton
                    href={ROUTES.propertyContactsNew(propertyId)}
                    variant="primary"
                    fullWidth
                    disabled={isArchived}
                    title={isArchived ? 'Объект в архиве' : undefined}
                >
                    Добавить контакт
                </LinkButton>
            )}

            {!isPending && !isError && contacts && contacts.length > 0 && (
                <>
                    <ul className={styles.list}>
                        {contacts.map((contact) => (
                            <li key={contact.id} className={styles.contact}>
                                <p className={styles.contactName}>{contact.name}</p>
                                <p className={styles.contactPhone}>{contact.phone}</p>
                            </li>
                        ))}
                    </ul>
                    <div className={styles.actions}>
                        <LinkButton
                            href={ROUTES.propertyContactsNew(propertyId)}
                            variant="secondary"
                            fullWidth
                            disabled={isArchived}
                            title={isArchived ? 'Объект в архиве' : undefined}
                        >
                            Добавить контакт
                        </LinkButton>
                    </div>
                </>
            )}
        </PropertyDetailSection>
    );
}
