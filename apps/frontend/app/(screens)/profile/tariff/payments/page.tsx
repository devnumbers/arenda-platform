import type { Metadata } from 'next';
import { PageContent, TopNav, TopNavBackButton, TopNavTitle } from '@/shared/ui/design';
import { ROUTES } from '@/shared/config/routes';
import { PaymentList } from '@/widgets/profile';

export const metadata: Metadata = {
  title: 'История платежей — Рентли',
  description: 'История платежей по тарифу',
};

export default function PaymentsPage() {
  return (
    <>
      <TopNav leading={<TopNavBackButton fallbackHref={ROUTES.profileTariff} />}>
        <TopNavTitle title="История платежей" />
      </TopNav>
      <PageContent className="px-6">
        <PaymentList />
      </PageContent>
    </>
  );
}
