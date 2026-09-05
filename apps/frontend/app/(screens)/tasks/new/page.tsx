import type { Metadata } from 'next';
import { TaskCreateScreen } from '@/widgets/tasks';

/** Экран «Создать задачу» с глобальной ленты (#525): та же форма, что на
 * объекте (#500), но без предвыбранного объекта — поле «Объект» пустое,
 * создание без выбора объекта уходит в безобъектный срез (POST /tasks/rules). */
export const metadata: Metadata = {
  title: 'Создать задачу — Рентли',
};

export default function GlobalTaskCreateRoutePage() {
  return <TaskCreateScreen initialPropertyId={null} />;
}
