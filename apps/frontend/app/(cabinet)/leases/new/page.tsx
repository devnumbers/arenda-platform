import type {Metadata} from 'next';
import {LeaseCreateWizard} from '@/widgets/leases';
import {PageShell} from '@/shared/ui/page-shell';

export const metadata: Metadata = {
    title: 'Создать аренду — Рентли',
};

type LeasesNewPageProps = {
    searchParams: Promise<Record<string, string | string[] | undefined>>;
};

export default async function LeasesNewPage({searchParams}: LeasesNewPageProps) {
    const params = await searchParams;
    const rawPropertyId = params.propertyId;
    const propertyId = Array.isArray(rawPropertyId) ? rawPropertyId[0] : rawPropertyId;
    const rawTenantContactId = params.tenantContactId;
    const tenantContactId = Array.isArray(rawTenantContactId) ? rawTenantContactId[0] : rawTenantContactId;

    return (
        <PageShell>
            <LeaseCreateWizard propertyId={propertyId} preselectedTenantContactId={tenantContactId}/>
        </PageShell>
    );
}
