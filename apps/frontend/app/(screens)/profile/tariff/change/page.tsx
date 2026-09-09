import type { Metadata } from 'next';
import { PageContent, TopNav, TopNavBackButton, TopNavTitle } from '@/shared/ui/design';
import { ROUTES } from '@/shared/config/routes';
import { TariffChangeForm } from '@/widgets/profile';

export const metadata: Metadata = {
  title: 'Сменить тариф — Рентли',
  description: 'Выбор нового тарифа и периода оплаты',
};

export default function TariffChangePage() {
  return (
    <>
      <TopNav leading={<TopNavBackButton fallbackHref={ROUTES.profileTariff} />}>
        <TopNavTitle title="Сменить тариф" />
      </TopNav>
      <PageContent className="px-6">
        <TariffChangeForm />
      </PageContent>
    </>
  );
}
