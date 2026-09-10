import type { Metadata } from 'next';
import { SubScreenShell } from '@/shared/ui/design';
import { ROUTES } from '@/shared/config/routes';
import { PersonalDataForm } from '@/widgets/profile';

export const metadata: Metadata = {
  title: 'Мои данные — Рентли',
  description: 'Редактирование персональных данных',
};

export default function PersonalDataPage() {
  return (
    <>
      <SubScreenShell title="Мои данные" fallbackHref={ROUTES.profile}>
        <PersonalDataForm />
      </SubScreenShell>
    </>
  );
}
