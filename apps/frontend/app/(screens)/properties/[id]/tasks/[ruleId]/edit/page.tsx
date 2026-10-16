import type { Metadata } from 'next';
import { TaskEditScreen } from '@/widgets/tasks';

/** Экран «Изменить задачу» (#502): одноэкранная форма правки правила —
 * будущие вхождения перематериализуются, выполненные остаются со снимком. */
export const metadata: Metadata = {
  title: 'Изменить задачу — Рентли',
};

export default async function TaskEditRoutePage({ params }: PageProps<'/properties/[id]/tasks/[ruleId]/edit'>) {
  const { id, ruleId } = await params;

  return <TaskEditScreen propertyId={id} ruleId={ruleId} />;
}
