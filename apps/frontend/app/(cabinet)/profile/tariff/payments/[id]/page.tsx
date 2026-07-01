import type { Metadata } from 'next';
import { PageHeader } from '@/shared/ui/page-header';
import { PageShell } from '@/shared/ui/page-shell';
import { ROUTES } from '@/shared/config/routes';
import { PaymentDetail } from '@/widgets/profile';

export const metadata: Metadata = {
  title: 'Операция — Рентли',
  description: 'Детали операции по тарифу',
};

export default async function PaymentDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;

  return (
    <PageShell>
      <PageHeader title="Операция" backHref={ROUTES.profilePayments} />
      <PaymentDetail id={id} />
    </PageShell>
  );
}
