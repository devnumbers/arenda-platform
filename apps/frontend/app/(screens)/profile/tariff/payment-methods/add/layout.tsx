import type { Metadata } from 'next';
import type { ReactNode } from 'react';

// Страница — клиентский компонент, metadata живут в серверном лейауте
// сегмента (паттерн app/login/layout.tsx).
export const metadata: Metadata = {
  title: 'Добавить карту — Рентли',
  description: 'Привязка банковской карты для оплаты тарифа',
};

export default function AddPaymentMethodLayout({
  children,
}: {
  readonly children: ReactNode;
}) {
  return children;
}
