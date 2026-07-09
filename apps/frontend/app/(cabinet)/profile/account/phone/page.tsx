import type { Metadata } from 'next';
import { PageShell } from '@/shared/ui/page-shell';
import { PhoneChangeForm } from '@/widgets/profile';

export const metadata: Metadata = {
  title: 'Изменение телефона — Рентли',
  description: 'Изменение номера телефона пользователя',
};

export default function ChangePhonePage() {
  return (
    <PageShell>
      <PhoneChangeForm />
    </PageShell>
  );
}
