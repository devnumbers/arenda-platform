import type {Metadata} from 'next';
import {PageShell} from '@/shared/ui/page-shell';
import {LeaseEditForm} from '@/widgets/leases';
import {sanitizeReturnTo} from '@/shared/lib/navigation';

export const metadata: Metadata = {
    title: 'Редактировать аренду — Рентли',
    description: 'Изменение информации об аренде',
};

export default async function LeaseEditPage({
    params,
    searchParams,
}: {
    params: Promise<{ id: string }>;
    searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
    const {id} = await params;
    const {returnTo, tenantContactId} = await searchParams;
    const sanitizedReturnTo = sanitizeReturnTo(typeof returnTo === 'string' ? returnTo : undefined);
    const preselectedTenantContactId = typeof tenantContactId === 'string' ? tenantContactId : undefined;

    return (
        <PageShell>
            <LeaseEditForm
                leaseId={id}
                returnTo={sanitizedReturnTo}
                preselectedTenantContactId={preselectedTenantContactId}
            />
        </PageShell>
    );
}
