import type { Metadata } from 'next';
import { Suspense } from 'react';
import { OperationsPage } from '@/widgets/operations/ui/OperationsPage';
import { FinanceLoading } from '@/widgets/finance/ui/FinanceLoading';

export const metadata: Metadata = {
  title: 'Операции — Arenda Platform',
  description: 'Список финансовых операций',
};

export default function FinanceOperationsPage() {
  return (
    <Suspense fallback={<FinanceLoading />}>
      <OperationsPage />
    </Suspense>
  );
}
