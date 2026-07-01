import type { Metadata } from 'next';
import { Suspense } from 'react';
import { PageShell } from '@/shared/ui/page-shell';
import { RecurringOperationEditPage } from '@/widgets/operations/ui/RecurringOperationEditPage';
import { FinanceLoading } from '@/widgets/finance/ui/FinanceLoading';

export const metadata: Metadata = {
  title: 'Редактирование серии — Arenda Platform',
};

export default function FinanceRecurringOperationEditPage() {
  return (
    <Suspense fallback={<FinanceLoading />}>
      <PageShell>
        <RecurringOperationEditPage />
      </PageShell>
    </Suspense>
  );
}
