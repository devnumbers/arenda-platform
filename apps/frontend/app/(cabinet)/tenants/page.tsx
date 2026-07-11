import type {Metadata} from 'next';
import {TenantsPage} from '@/widgets/tenants';
import {PageShell} from '@/shared/ui/page-shell';

export const metadata: Metadata = {
    title: 'Арендаторы — Рентли',
    description: 'Список арендаторов',
};

export default function TenantsListPage() {
    return (
        <PageShell>
            <TenantsPage/>
        </PageShell>
    );
}