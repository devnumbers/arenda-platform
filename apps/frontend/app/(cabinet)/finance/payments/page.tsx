import type { Metadata } from 'next';
import { Suspense } from 'react';
import { PaymentsPage } from '@/widgets/operations/ui/PaymentsPage';
import { FinanceLoading } from '@/widgets/finance/ui/FinanceLoading';

export const metadata: Metadata = {
  title: 'Платежи — Arenda Platform',
  description: 'Запланированные и просроченные платежи',
};

export default function FinancePaymentsPage() {
  return (
    <Suspense fallback={<FinanceLoading />}>
      <PaymentsPage />
    </Suspense>
  );
}
