import type { Metadata } from 'next';
import { DashboardPage } from '@/widgets/dashboard';

export const metadata: Metadata = {
  title: 'Главная — Arenda Platform',
  description: 'Главная страница личного кабинета',
};

export default function DashboardRoutePage() {
  return <DashboardPage />;
}
