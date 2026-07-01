import type { Metadata } from 'next';
import { Suspense } from 'react';
import { PageShell } from '@/shared/ui/page-shell';
import { OperationEditForm } from '@/widgets/operations';
import { FinanceLoading } from '@/widgets/finance/ui/FinanceLoading';

export const metadata: Metadata = {
  title: 'Редактирование операции — Рентли',
};

export default function FinanceOperationEditPage() {
  return (
    <Suspense fallback={<FinanceLoading />}>
      <PageShell>
        <OperationEditForm />
      </PageShell>
    </Suspense>
  );
}
