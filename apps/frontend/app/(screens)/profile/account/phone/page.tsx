import type { Metadata } from 'next';
import { PageContent, TopNav, TopNavBackButton, TopNavTitle } from '@/shared/ui/design';
import { ROUTES } from '@/shared/config/routes';
import { PhoneChangeForm } from '@/widgets/profile';

export const metadata: Metadata = {
  title: 'Изменение телефона — Рентли',
  description: 'Изменение номера телефона пользователя',
};

export default function ChangePhonePage() {
  return (
    <>
      <TopNav leading={<TopNavBackButton fallbackHref={ROUTES.profileAccount} />}>
        <TopNavTitle title="Изменение телефона" />
      </TopNav>
      <PageContent className="px-6">
        <PhoneChangeForm />
      </PageContent>
    </>
  );
}
