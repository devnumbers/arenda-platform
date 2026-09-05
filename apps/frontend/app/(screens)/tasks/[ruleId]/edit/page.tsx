import type { Metadata } from 'next';
import { TaskPropertylessEditScreen } from '@/widgets/tasks';

/** Плоский маршрут «Изменить задачу» (#537): правка правила без объекта
 * (ADR 0052) — та же форма, что у объекта (#502); тап по активной
 * безобъектной строке глобальной ленты (#523). */
export const metadata: Metadata = {
  title: 'Изменить задачу — Рентли',
};

type TaskPropertylessEditRoutePageProps = {
  params: Promise<{ ruleId: string }>;
};

export default async function TaskPropertylessEditRoutePage({
  params,
}: TaskPropertylessEditRoutePageProps) {
  const { ruleId } = await params;

  return <TaskPropertylessEditScreen ruleId={ruleId} />;
}
