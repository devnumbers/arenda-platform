import type { Metadata } from 'next';
import { PageContent, TopNav, TopNavBackButton, TopNavTitle } from '@/shared/ui/design';
import { ROUTES } from '@/shared/config/routes';
import { AccountOverview } from '@/widgets/profile';

export const metadata: Metadata = {
  title: 'Аккаунт — Рентли',
  description: 'Управление аккаунтом пользователя',
};

export default function AccountPage() {
  return (
    <>
      <TopNav leading={<TopNavBackButton fallbackHref={ROUTES.profile} />}>
        <TopNavTitle title="Аккаунт" />
      </TopNav>
      <PageContent className="px-6">
        <AccountOverview />
      </PageContent>
    </>
  );
}
