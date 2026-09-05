import type { Metadata } from 'next';
import { OperationsGlobalScreen } from '@/widgets/payments';

/** Экран «Операции» — глобальная лента по всем объектам (карта #545,
 * тикет #541): живёт в кабинете, вход — пункт бокового меню, только ПК
 * (решение владельца #539; мобильный вход — вне скоупа). */
export const metadata: Metadata = {
  title: 'Операции — Рентли',
};

export default function OperationsRoutePage() {
  return <OperationsGlobalScreen />;
}
