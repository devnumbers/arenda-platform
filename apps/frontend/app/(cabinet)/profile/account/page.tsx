import type { Metadata } from 'next';
import { PageShell } from '@/shared/ui/page-shell';
import { AccountOverview } from '@/widgets/profile';

export const metadata: Metadata = {
  title: 'Аккаунт — Рентли',
  description: 'Управление аккаунтом пользователя',
};

export default function AccountPage() {
  return (
    <PageShell>
      <AccountOverview />
    </PageShell>
  );
}
