import type { Metadata } from 'next';
import { PageHeader } from '@/shared/ui/page-header';
import { PageShell } from '@/shared/ui/page-shell';
import { ProfileOverview } from '@/widgets/profile';

export const metadata: Metadata = {
  title: 'Профиль — Arenda Platform',
  description: 'Страница профиля пользователя',
};

export default function ProfilePage() {
  return (
    <PageShell>
      <PageHeader title="Профиль" />
      <ProfileOverview />
    </PageShell>
  );
}
