import type { Metadata } from 'next';
import { PageContent, TopNav, TopNavBackButton, TopNavTitle } from '@/shared/ui/design';
import { ROUTES } from '@/shared/config/routes';
import { PersonalDataForm } from '@/widgets/profile';

export const metadata: Metadata = {
  title: 'Мои данные — Рентли',
  description: 'Редактирование персональных данных',
};

export default function PersonalDataPage() {
  return (
    <>
      <TopNav leading={<TopNavBackButton fallbackHref={ROUTES.profile} />}>
        <TopNavTitle title="Мои данные" />
      </TopNav>
      <PageContent className="px-6">
        <PersonalDataForm />
      </PageContent>
    </>
  );
}
