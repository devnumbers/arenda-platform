import type { Metadata } from 'next';
import { PageShell } from '@/shared/ui/page-shell';
import { PersonalDataForm } from '@/widgets/profile';

export const metadata: Metadata = {
  title: 'Мои данные — Рентли',
  description: 'Редактирование персональных данных',
};

export default function PersonalDataPage() {
  return (
    <PageShell>
      <PersonalDataForm />
    </PageShell>
  );
}
