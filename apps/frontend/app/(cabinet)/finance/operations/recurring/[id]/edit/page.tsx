import type { Metadata } from 'next';
import { Suspense } from 'react';
import { PageShell } from '@/shared/ui/page-shell';
import { RecurringOperationEditPage } from '@/widgets/operations';
import { FinanceLoading } from '@/shared/ui/finance-loading';

export const metadata: Metadata = {
  title: 'Редактирование серии — Рентли',
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
