import type { Metadata } from 'next';
import { ProjectedOperationScreen } from '@/widgets/payments';

/**
 * Просмотр проекции будущего вхождения «Графика» (чисто фронт, без записи
 * в БД): валидность даты сверяется с расписанием правила на экране.
 */

export const metadata: Metadata = {
  title: 'Операция — Рентли',
};

type ProjectedOperationRoutePageProps = {
  params: Promise<{ id: string; paymentId: string; date: string }>;
};

export default async function ProjectedOperationRoutePage({
  params,
}: ProjectedOperationRoutePageProps) {
  const { id, paymentId, date } = await params;

  return <ProjectedOperationScreen propertyId={id} paymentId={paymentId} date={date} />;
}
