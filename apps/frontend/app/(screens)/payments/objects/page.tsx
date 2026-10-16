import type { Metadata } from 'next';
import { PaymentsObjectsScreen } from '@/widgets/payments';

/**
 * Страница «Объекты» — ленд секции «Платежи объектов» (карта #573, тикет
 * #582, Figma 654:7558): карточки объектов со стопками правил, булавка-
 * индикатор скрепления, поиск объектов и «Архивные объекты».
 */

export const metadata: Metadata = {
  title: 'Объекты — Рентли',
};

export default function PaymentsObjectsRoutePage() {
  return <PaymentsObjectsScreen />;
}
