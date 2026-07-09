import type { Metadata } from 'next';
import { PageShell } from '@/shared/ui/page-shell';
import { TariffChangeForm } from '@/widgets/profile';

export const metadata: Metadata = {
  title: 'Сменить тариф — Рентли',
  description: 'Выбор нового тарифа и периода оплаты',
};

export default function TariffChangePage() {
  return (
    <PageShell>
      <TariffChangeForm />
    </PageShell>
  );
}
