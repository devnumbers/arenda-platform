import type { Metadata } from 'next';
import { PaymentsOfPropertyScreen } from '@/widgets/payments';

/** Экран «Платежи объекта» (#463): секции «Просроченные», «Платежи»,
 * «Автоплатежи», поиск, шит выбора «Платёж / Автоплатёж». Оболочка новых
 * экранов (глобальный top-header на десктопе, колонка 560) — из layout
 * группы (screens). */
export const metadata: Metadata = {
  title: 'Платежи объекта — Рентли',
};

type PaymentsRoutePageProps = {
  params: Promise<{ id: string }>;
};

export default async function PaymentsRoutePage({ params }: PaymentsRoutePageProps) {
  const { id } = await params;

  return <PaymentsOfPropertyScreen propertyId={id} />;
}
