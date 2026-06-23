import type { Metadata } from 'next';

export const metadata: Metadata = {
  title: 'Главная — Arenda Platform',
  description: 'Главная страница личного кабинета',
};

export default function DashboardPage() {
  return <h1>Главная</h1>;
}
