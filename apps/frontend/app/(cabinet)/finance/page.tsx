import type { Metadata } from 'next';
import { FinancePage } from '@/widgets/finance/ui/FinancePage';

export const metadata: Metadata = {
  title: 'Финансы — Рентли',
  description: 'Страница финансов',
};

export default function FinanceCabinetPage() {
  return <FinancePage />;
}
