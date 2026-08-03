import type { Metadata } from 'next';
import { PageShell } from '@/shared/ui/page-shell';
import { FreeReminderDetailPage } from '@/widgets/free-reminders';

export const metadata: Metadata = {
  title: 'Напоминание — Рентли',
  description: 'Просмотр свободного напоминания',
};

export default function FreeReminderDetailRoute() {
  return (
    <PageShell>
      <FreeReminderDetailPage />
    </PageShell>
  );
}
