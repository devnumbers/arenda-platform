import type { Metadata } from 'next';
import type { JSX } from 'react';
import { PageShell } from '@/shared/ui/page-shell';
import { CalendarPage } from '@/widgets/calendar';

export const metadata: Metadata = {
  title: 'Календарь — Рентли',
  description: 'Все напоминания собственника: свободные, по операциям и системные',
};

export default function CalendarRoutePage(): JSX.Element {
  return (
    <PageShell>
      <CalendarPage />
    </PageShell>
  );
}
