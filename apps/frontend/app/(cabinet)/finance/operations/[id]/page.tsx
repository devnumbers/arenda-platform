import type { Metadata } from 'next';
import { Suspense } from 'react';
import { PageShell } from '@/shared/ui/page-shell';
import { OperationDetailPage } from '@/widgets/operations/ui/OperationDetailPage';
import { FinanceLoading } from '@/widgets/finance/ui/FinanceLoading';

export const metadata: Metadata = {
  title: 'Операция — Arenda Platform',
  description: 'Детали финансовой операции',
};

export default function FinanceOperationDetailPage() {
  return (
    <Suspense fallback={<FinanceLoading />}>
      <PageShell>
        <OperationDetailPage />
      </PageShell>
    </Suspense>
  );
}
