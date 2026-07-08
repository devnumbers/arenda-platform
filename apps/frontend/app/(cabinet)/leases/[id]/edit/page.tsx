import type {Metadata} from 'next';
import {PageShell} from '@/shared/ui/page-shell';
import {LeaseEditForm} from '@/widgets/leases';

export const metadata: Metadata = {
    title: 'Редактировать аренду — Рентли',
    description: 'Изменение информации об аренде',
};

export default async function LeaseEditPage({params}: {params: Promise<{ id: string }>}) {
    const {id} = await params;
    return (
        <PageShell>
            <LeaseEditForm leaseId={id}/>
        </PageShell>
    );
}
