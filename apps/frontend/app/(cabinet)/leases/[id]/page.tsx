import type {Metadata} from 'next';
import {LeaseDetailPage} from '@/widgets/leases';
import {PageShell} from '@/shared/ui/page-shell';

export const metadata: Metadata = {
    title: 'Аренда — Рентли',
    description: 'Просмотр и редактирование аренды',
};

export default async function LeasePage({
                                            params,
                                        }: {
    params: Promise<{ id: string }>;
}) {
    const {id} = await params;
    return (
        <PageShell>
            <LeaseDetailPage id={id}/>
        </PageShell>
    );
}
