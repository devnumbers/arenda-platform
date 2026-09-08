import type { Metadata } from 'next';
import { PaymentsStubScreen } from '@/widgets/payments';

/**
 * Глобальные «Платежи» — страница-заглушка единого хрома (карта #556,
 * тикет #559): пункт «Платежи» главной навигации ведёт на живой маршрут,
 * содержание раздела — отдельное усилие владельца.
 */
export const metadata: Metadata = {
  title: 'Платежи — Рентли',
};

export default function PaymentsRoutePage() {
  return <PaymentsStubScreen />;
}
