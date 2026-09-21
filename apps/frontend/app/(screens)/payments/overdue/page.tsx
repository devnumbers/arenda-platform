import type { Metadata } from 'next';
import {
  PaymentOverdueGlobalScreen,
  parseOverdueSortParams,
} from '@/widgets/payments';

/** Глобальная страница «Просроченные операции» (карта #573, тикет #580).
 * Единый хром — ScreenLayout группы (screens) (карта #556); вход — карточка
 * «Все просроченные» главного экрана платежей. Направление сортировки живёт в
 * query строки (?sort=new, дефолт «Старые» не пишется) — переживает
 * перезагрузку. */
export const metadata: Metadata = {
  title: 'Просроченные операции — Рентли',
};

export default async function PaymentOverdueRoutePage({
  searchParams,
}: PageProps<'/payments/overdue'>) {
  const resolved = await searchParams;
  const sort = parseOverdueSortParams(resolved.sort);

  return <PaymentOverdueGlobalScreen initialSort={sort} />;
}
