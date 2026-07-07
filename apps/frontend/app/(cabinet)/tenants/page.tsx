import type {Metadata} from 'next';
import {TenantsPage} from '@/widgets/tenants';
import {PageShell} from "@/shared/ui/page-shell";
import {PageHeader} from "@/shared/ui/page-header";

export const metadata: Metadata = {
    title: 'Арендаторы — Рентли',
    description: 'Список арендаторов',
};

export default function TenantsListPage() {
    return (
        <PageShell>
            <PageHeader title="Арендаторы"/>
            <TenantsPage/>
        </PageShell>
    )
}