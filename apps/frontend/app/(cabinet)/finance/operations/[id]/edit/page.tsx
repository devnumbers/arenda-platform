import type { Metadata } from 'next';
import { Suspense } from 'react';
import { OperationEditForm } from '@/widgets/operations';
import { FinanceLoading } from '@/widgets/finance/ui/FinanceLoading';

export const metadata: Metadata = {
  title: 'Редактирование операции — Arenda Platform',
};

export default function FinanceOperationEditPage() {
  return (
    <Suspense fallback={<FinanceLoading />}>
      <OperationEditForm />
    </Suspense>
  );
}
