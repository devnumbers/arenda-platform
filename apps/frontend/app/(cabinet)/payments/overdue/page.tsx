import type { Metadata } from 'next';
import {
  PaymentOverdueGlobalScreen,
  parseOverdueSortParams,
} from '@/widgets/payments';

/** Глобальная страница «Просроченные операции» (карта #573, тикет #580).
 * Оболочка кабинета — из layout группы (cabinet); вход — карточка «Все
 * просроченные» главного экрана платежей. Направление сортировки живёт в
 * query строки (?sort=new, дефолт «Старые» не пишется) — переживает
 * перезагрузку. */
export const metadata: Metadata = {
  title: 'Просроченные операции — Рентли',
};

type OverdueRoutePageProps = {
  searchParams?: Promise<Record<string, string | string[] | undefined>>;
};

export default async function PaymentOverdueRoutePage({
  searchParams,
}: OverdueRoutePageProps) {
  const resolved = searchParams ? await searchParams : {};
  const sort = parseOverdueSortParams(resolved.sort);

  return <PaymentOverdueGlobalScreen initialSort={sort} />;
}
