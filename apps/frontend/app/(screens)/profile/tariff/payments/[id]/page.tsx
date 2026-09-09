import type { Metadata } from 'next';
import { PageContent, TopNav, TopNavBackButton, TopNavTitle } from '@/shared/ui/design';
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
      <TopNav leading={<TopNavBackButton fallbackHref={ROUTES.profilePayments} />}>
        <TopNavTitle title="Платёж" />
      </TopNav>
      <PageContent className="px-6">
        <PaymentDetail id={id} />
      </PageContent>
    </>
  );
}
