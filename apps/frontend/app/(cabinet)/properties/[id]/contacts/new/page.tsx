import type {Metadata} from 'next';
import {PropertyContactCreatePage} from '@/widgets/property-detail';
import {PageShell} from '@/shared/ui/page-shell';

export const metadata: Metadata = {
    title: 'Новый контакт — Рентли',
    description: 'Добавление контакта объекта',
};

interface PropertyContactNewPageProps {
    params: Promise<{ id: string }>;
}

export default async function PropertyContactNewPage({params}: PropertyContactNewPageProps) {
    const {id} = await params;

    return (
        <PageShell>
            <PropertyContactCreatePage propertyId={id}/>
        </PageShell>
    );
}
