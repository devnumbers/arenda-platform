import type { Metadata } from 'next';
import { SubScreenShell } from '@/shared/ui/design';
import { ROUTES } from '@/shared/config/routes';
import { PhoneChangeForm } from '@/widgets/profile';

export const metadata: Metadata = {
  title: 'Изменение телефона — Рентли',
  description: 'Изменение номера телефона пользователя',
};

export default function ChangePhonePage() {
  return (
    <>
      <SubScreenShell title="Изменение телефона" fallbackHref={ROUTES.profileAccount}>
        <PhoneChangeForm />
      </SubScreenShell>
    </>
  );
}
