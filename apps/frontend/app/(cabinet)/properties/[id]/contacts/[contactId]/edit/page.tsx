import type {Metadata} from 'next';
import {PropertyContactEditPage} from '@/widgets/property-detail';
import {PageShell} from '@/shared/ui/page-shell';

export const metadata: Metadata = {
    title: 'Редактирование контакта — Рентли',
    description: 'Редактирование контакта объекта',
};

interface PropertyContactEditPageProps {
    params: Promise<{ id: string; contactId: string }>;
}

export default async function PropertyContactEditRoutePage(
    {params}: PropertyContactEditPageProps,
) {
    const {id, contactId} = await params;

    return (
        <PageShell>
            <PropertyContactEditPage propertyId={id} contactId={contactId}/>
        </PageShell>
    );
}
