import type { Metadata } from 'next';
import { TaskCreateScreen } from '@/widgets/tasks';

/** Экран «Создать задачу» (#500): полноэкранная форма в два шага —
 * название, затем комментарий/дата/время/повтор. Закрытие — ✕, создание —
 * POST /tasks/rules с возвратом в список. */
export const metadata: Metadata = {
  title: 'Создать задачу — Рентли',
};

type TaskCreateRoutePageProps = {
  params: Promise<{ id: string }>;
};

export default async function TaskCreateRoutePage({ params }: TaskCreateRoutePageProps) {
  const { id } = await params;

  return <TaskCreateScreen propertyId={id} />;
}
