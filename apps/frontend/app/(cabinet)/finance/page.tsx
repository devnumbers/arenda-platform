import type { Metadata } from 'next';
import { FinancePage } from '@/widgets/finance/ui/FinancePage';

export const metadata: Metadata = {
  title: 'Финансы — Arenda Platform',
  description: 'Страница финансов',
};

export default function FinanceCabinetPage() {
  return <FinancePage />;
}
