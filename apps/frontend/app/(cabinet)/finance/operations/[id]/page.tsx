import type { Metadata } from 'next';
import { PageShell } from '@/shared/ui/page-shell';
import { OperationDetailPage } from '@/widgets/operations';

export const metadata: Metadata = {
  title: 'Операция — Рентли',
  description: 'Детали финансовой операции',
};

export default function FinanceOperationDetailPage() {
  return (
    <PageShell>
      <OperationDetailPage />
    </PageShell>
  );
}
