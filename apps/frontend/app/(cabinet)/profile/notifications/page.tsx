import type { Metadata } from 'next';
import { PageShell } from '@/shared/ui/page-shell';
import { NotificationSettings } from '@/widgets/profile';

export const metadata: Metadata = {
  title: 'Уведомления — Рентли',
  description: 'Управление настройками уведомлений',
};

export default function NotificationsPage() {
  return (
    <PageShell>
      <NotificationSettings />
    </PageShell>
  );
}
