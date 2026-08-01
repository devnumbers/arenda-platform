'use client';

import {useCallback, type JSX} from 'react';
import {useRouter} from 'next/navigation';
import {notify} from '@/shared/lib/notifications';
import {goBack} from '@/shared/lib/navigation';
import {ROUTES} from '@/shared/config/routes';
import {useCreatePropertyContact} from '@/features/property-contacts/api';
import {PageHeader} from '@/shared/ui/page-header';
import {PropertyContactForm, type PropertyContactFormData} from './PropertyContactForm';

export type PropertyContactCreatePageProps = {
    readonly propertyId: string;
};

export function PropertyContactCreatePage({
                                             propertyId,
                                         }: PropertyContactCreatePageProps): JSX.Element {
    const router = useRouter();
    const createContact = useCreatePropertyContact(propertyId);

    const handleSubmit = useCallback(
        async (data: PropertyContactFormData) => {
            try {
                await createContact.mutateAsync(data);
                notify.scenarios.propertyContacts.created();
                goBack(router, ROUTES.property(propertyId));
            } catch (error: unknown) {
                notify.scenarios.propertyContacts.createError(error);
            }
        },
        [createContact, propertyId, router],
    );

    const handleCancel = useCallback(() => {
        goBack(router, ROUTES.property(propertyId));
    }, [propertyId, router]);

    return (
        <>
            <PageHeader
                title="Новый контакт"
                backHref={ROUTES.property(propertyId)}
            />

            <PropertyContactForm
                submitLabel="Добавить контакт"
                isLoading={createContact.isPending}
                onSubmit={handleSubmit}
                onCancel={handleCancel}
            />
        </>
    );
}
