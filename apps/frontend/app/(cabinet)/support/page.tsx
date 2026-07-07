import type { Metadata } from 'next';
import { SupportPage } from '@/widgets/support';

export const metadata: Metadata = {
  title: 'Поддержка — Рентли',
  description: 'Контакты службы поддержки',
};

export default function SupportRoute() {
  return <SupportPage />;
}
