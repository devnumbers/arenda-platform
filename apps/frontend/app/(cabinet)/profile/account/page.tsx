import type { Metadata } from 'next';
import { PageHeader } from '@/shared/ui/page-header';
import { PageShell } from '@/shared/ui/page-shell';
import { ROUTES } from '@/shared/config/routes';
import { AccountOverview } from '@/widgets/profile';

export const metadata: Metadata = {
  title: 'Аккаунт — Arenda Platform',
  description: 'Управление аккаунтом пользователя',
};

export default function AccountPage() {
  return (
    <PageShell>
      <PageHeader title="Аккаунт" backHref={ROUTES.profile} />
      <AccountOverview />
    </PageShell>
  );
}
