import type { Metadata } from 'next';
import { SupportScreen } from '@/widgets/support';

// Кабинетная страница: куки-сессия — всегда динамический рендер.
export const dynamic = 'force-dynamic';

export const metadata: Metadata = {
  title: 'Поддержка — Рентли',
  description: 'Свяжитесь с нами: Телеграм или почта',
};

/** Хаб «Поддержка» (макеты 40000074:80636/40000075:80950, 28.10):
 * страница восстановлена вместо модалки #766 — пилюля ПК и шестая
 * ячейка шита «Еще» ведут сюда. */
export default function SupportPage() {
  return <SupportScreen />;
}
