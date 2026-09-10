import type { Metadata } from 'next';
import { SubScreenShell } from '@/shared/ui/design';
import { ROUTES } from '@/shared/config/routes';
import { PaymentDetail } from '@/widgets/profile';

export const metadata: Metadata = {
  title: 'Платёж — Рентли',
  description: 'Детали платежа по тарифу',
};

export default async function PaymentDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;

  return (
    <>
      <SubScreenShell title="Платёж" fallbackHref={ROUTES.profilePayments}>
        <PaymentDetail id={id} />
      </SubScreenShell>
    </>
  );
}
