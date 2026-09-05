import type { Metadata } from 'next';
import { TasksFeedScreen } from '@/widgets/tasks';

/** Глобальная лента «Задачи» (#523, Figma 1733-27411/1726-86913/1733-92349):
 * топ-уровень группы (screens) рядом с объектами; вход — пункт «Задачи» в
 * меню профиля (решение 1 #522). */
export const metadata: Metadata = {
  title: 'Задачи — Рентли',
};

export default function TasksRoutePage() {
  return <TasksFeedScreen />;
}
