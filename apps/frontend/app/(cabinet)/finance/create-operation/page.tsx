import type { Metadata } from 'next';
import { PageShell } from '@/shared/ui/page-shell';
import { OperationCreateWizard } from '@/widgets/operations';

export const metadata: Metadata = {
  title: 'Создание операции — Arenda Platform',
  description: 'Создание операции дохода или расхода',
};

export default async function CreateOperationPage({
  searchParams,
}: {
  searchParams: Promise<{ type?: string; propertyId?: string | string[] }>;
}) {
  const { type, propertyId } = await searchParams;
  const operationType = type === 'income' || type === 'expense' ? type : 'expense';
  const selectedPropertyId = typeof propertyId === 'string' ? propertyId : undefined;

  return (
    <PageShell>
      <OperationCreateWizard type={operationType} propertyId={selectedPropertyId} />
    </PageShell>
  );
}
