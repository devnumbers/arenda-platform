import type {Metadata} from 'next';
import {PageShell} from '@/shared/ui/page-shell';
import {PropertyAccessPage} from '@/widgets/property-detail';

export const metadata: Metadata = {
    title: 'Совместный доступ — Рентли',
    description: 'Управление участниками совместного доступа к объекту',
};

interface PropertyAccessRouteProps {
    params: Promise<{ id: string }>;
}

export default async function PropertyAccessRoutePage({params}: PropertyAccessRouteProps) {
    // The property id is read from the URL by the client widget via useParams,
    // so we only await params to satisfy the dynamic route contract.
    await params;

    return (
        <PageShell>
            <PropertyAccessPage/>
        </PageShell>
    );
}
