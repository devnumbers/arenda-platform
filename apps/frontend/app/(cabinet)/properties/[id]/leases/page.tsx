import type {Metadata} from 'next';
import {Suspense} from 'react';
import {PageShell} from '@/shared/ui/page-shell';
import {PropertyLeasesPage} from '@/widgets/property-detail';
import {FinanceLoading} from '@/shared/ui/finance-loading';

export const metadata: Metadata = {
    title: 'Аренды объекта — Рентли',
    description: 'История аренд по объекту недвижимости',
};

export default function PropertyLeasesRoutePage() {
    return (
        <PageShell>
            <Suspense fallback={<FinanceLoading/>}>
                <PropertyLeasesPage/>
            </Suspense>
        </PageShell>
    );
}
