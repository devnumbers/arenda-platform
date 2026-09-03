import type { Metadata } from 'next';
import { TaskEditScreen } from '@/widgets/tasks';

/** Экран «Изменить задачу» (#502): одноэкранная форма правки правила —
 * будущие вхождения перематериализуются, выполненные остаются со снимком. */
export const metadata: Metadata = {
  title: 'Изменить задачу — Рентли',
};

type TaskEditRoutePageProps = {
  params: Promise<{ id: string; ruleId: string }>;
};

export default async function TaskEditRoutePage({ params }: TaskEditRoutePageProps) {
  const { id, ruleId } = await params;

  return <TaskEditScreen propertyId={id} ruleId={ruleId} />;
}
