import type { Metadata } from 'next';
import { SubScreenShell } from '@/shared/ui/design';
import { ROUTES } from '@/shared/config/routes';
import { AccountOverview } from '@/widgets/profile';

export const metadata: Metadata = {
  title: 'Аккаунт — Рентли',
  description: 'Управление аккаунтом пользователя',
};

export default function AccountPage() {
  return (
    <>
      <SubScreenShell title="Аккаунт" fallbackHref={ROUTES.profile}>
        <AccountOverview />
      </SubScreenShell>
    </>
  );
}
