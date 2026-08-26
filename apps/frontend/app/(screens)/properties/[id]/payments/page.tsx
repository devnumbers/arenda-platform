import type { Metadata } from 'next';
import { PaymentsStubScreen } from './stub-screen';

/** Заглушка экрана «Платежи объекта» (#460): демонстрирует оболочку
 * новых экранов (глобальный top-header на десктопе, локальный TopNav,
 * колонка 560) и служит шаблоном для экранных тикетов #463+. */
export const metadata: Metadata = {
  title: 'Платежи объекта — Рентли',
};

type PaymentsRoutePageProps = {
  params: Promise<{ id: string }>;
};

export default async function PaymentsRoutePage({ params }: PaymentsRoutePageProps) {
  const { id } = await params;

  return <PaymentsStubScreen propertyId={id} />;
}
