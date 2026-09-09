import type { Metadata } from 'next';
import { PageContent, TopNav, TopNavBackButton, TopNavTitle } from '@/shared/ui/design';
import { ROUTES } from '@/shared/config/routes';
import { TariffOverview } from '@/widgets/profile';

export const metadata: Metadata = {
  title: 'Тариф — Рентли',
  description: 'Управление тарифом и подпиской',
};

export default function TariffPage() {
  return (
    <>
      <TopNav leading={<TopNavBackButton fallbackHref={ROUTES.profile} />}>
        <TopNavTitle title="Тариф" />
      </TopNav>
      <PageContent className="px-6">
        <TariffOverview />
      </PageContent>
    </>
  );
}
