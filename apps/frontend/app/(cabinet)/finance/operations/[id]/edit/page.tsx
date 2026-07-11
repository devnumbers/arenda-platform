import type { Metadata } from 'next';
import { PageShell } from '@/shared/ui/page-shell';
import { OperationEditForm } from '@/widgets/operations';

export const metadata: Metadata = {
  title: 'Редактирование операции — Рентли',
};

export default function FinanceOperationEditPage() {
  return (
    <PageShell>
      <OperationEditForm />
    </PageShell>
  );
}
