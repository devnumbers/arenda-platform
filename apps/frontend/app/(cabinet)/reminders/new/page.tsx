import type { Metadata } from 'next';
import { PageShell } from '@/shared/ui/page-shell';
import { FreeReminderWizard } from '@/widgets/free-reminders';

export const metadata: Metadata = {
  title: 'Новое напоминание — Рентли',
  description: 'Создание свободного напоминания',
};

export default async function NewFreeReminderPage({
  searchParams,
}: {
  searchParams: Promise<{ propertyId?: string | string[] }>;
}) {
  const { propertyId } = await searchParams;
  const selectedPropertyId = typeof propertyId === 'string' ? propertyId : undefined;

  return (
    <PageShell>
      <FreeReminderWizard mode="create" propertyId={selectedPropertyId} />
    </PageShell>
  );
}
