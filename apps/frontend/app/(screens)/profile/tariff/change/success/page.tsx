import { Suspense } from 'react';
import type { Metadata } from 'next';
import { PageContent, TopNav, TopNavBackButton, TopNavTitle } from '@/shared/ui/design';
import { ROUTES } from '@/shared/config/routes';
import { TariffChangeSuccess } from '@/widgets/profile';

export const metadata: Metadata = {
  title: 'Тариф изменён — Рентли',
  description: 'Подтверждение смены тарифа',
};

export default function TariffChangeSuccessPage() {
  return (
    <>
      <TopNav leading={<TopNavBackButton fallbackHref={ROUTES.profileTariff} />}>
        <TopNavTitle title="Тариф изменён" />
      </TopNav>
      <PageContent className="px-6">
        <Suspense fallback={null}>
          <TariffChangeSuccess />
        </Suspense>
      </PageContent>
    </>
  );
}
