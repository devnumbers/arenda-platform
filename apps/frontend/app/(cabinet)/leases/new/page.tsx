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

    return (
        <PageShell>
            <LeaseCreateWizard propertyId={propertyId}/>
        </PageShell>
    );
}
