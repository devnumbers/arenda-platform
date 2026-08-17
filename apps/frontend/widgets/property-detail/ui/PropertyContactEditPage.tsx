'use client';

import {useCallback, useState, type JSX} from 'react';
import {useRouter} from 'next/navigation';
import {notify} from '@/shared/lib/notifications';
import {goBack} from '@/shared/lib/navigation';
import {ROUTES} from '@/shared/config/routes';
import {
    usePropertyContact,
    useUpdatePropertyContact,
    useDeletePropertyContact,
} from '@/features/property-contacts';
import {PageHeader} from '@/shared/ui/page-header';
import {IconButton} from '@/shared/ui/icon-button';
import {Button} from '@/shared/ui/button';
import {ConfirmModal} from '@/shared/ui/confirm-modal';
import {Cancel, Trash} from '@/shared/assets/icons';
import {PropertyContactForm, type PropertyContactFormData} from './PropertyContactForm';

export type PropertyContactEditPageProps = {
    readonly propertyId: string;
    readonly contactId: string;
};

export function PropertyContactEditPage({
                                            propertyId,
                                            contactId,
                                        }: PropertyContactEditPageProps): JSX.Element {
    const router = useRouter();
    const {data: contact, isPending} = usePropertyContact(propertyId, contactId);
    const updateContact = useUpdatePropertyContact(propertyId, contactId);
    const deleteContact = useDeletePropertyContact(propertyId);
    const [isDeleteModalOpen, setIsDeleteModalOpen] = useState(false);

    const handleSubmit = useCallback(
        async (data: PropertyContactFormData) => {
            try {
                await updateContact.mutateAsync(data);
                notify.scenarios.propertyContacts.updated();
                goBack(router, ROUTES.property(propertyId));
            } catch (error: unknown) {
                notify.scenarios.propertyContacts.updateError(error);
            }
        },
        [propertyId, router, updateContact],
    );

    const handleCancel = useCallback(() => {
        goBack(router, ROUTES.property(propertyId));
    }, [propertyId, router]);

    const handleDeleteClick = useCallback(() => {
        setIsDeleteModalOpen(true);
    }, []);

    const handleDeleteModalClose = useCallback(() => {
        setIsDeleteModalOpen(false);
    }, []);

    const handleDeleteConfirm = useCallback(async () => {
        try {
            await deleteContact.mutateAsync(contactId);
            notify.scenarios.propertyContacts.deleted();
            router.push(ROUTES.property(propertyId));
        } catch (error: unknown) {
            notify.scenarios.propertyContacts.deleteError(error);
        }
    }, [contactId, deleteContact, propertyId, router]);

    return (
        <>
            <PageHeader
                title="Редактирование контакта"
                backHref={ROUTES.property(propertyId)}
                actions={
                    <IconButton
                        variant="secondary"
                        size="large"
                        icon={<Cancel/>}
                        aria-label="Отменить"
                        onClick={handleCancel}
                    />
                }
            />

            {isPending && (
                <p>Загрузка…</p>
            )}

            {contact && (
                <PropertyContactForm
                    submitLabel="Сохранить изменения"
                    isLoading={updateContact.isPending}
                    onSubmit={handleSubmit}
                    initialValues={{
                        name: contact.name,
                        phone: contact.phone,
                    }}
                    extraActions={
                        <Button
                            type="button"
                            variant="secondary"
                            size="large"
                            fullWidth
                            leftIcon={<Trash/>}
                            loading={deleteContact.isPending}
                            onClick={handleDeleteClick}
                        >
                            Удалить контакт
                        </Button>
                    }
                />
            )}

            <ConfirmModal
                isOpen={isDeleteModalOpen}
                title="Удалить контакт?"
                description="Действие нельзя отменить."
                confirmLabel="Удалить"
                onClose={handleDeleteModalClose}
                onConfirm={handleDeleteConfirm}
            />
        </>
    );
}
