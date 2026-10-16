import type { Metadata } from 'next';
import { TaskCreateScreen } from '@/widgets/tasks';

/** Экран «Создать задачу» (#500): полноэкранная форма в два шага —
 * название, затем комментарий/дата/время/повтор. Вход с объекта — объект
 * предвыбран (#525), смена/очистка — на странице «Выбрать объект».
 * Закрытие — ✕, создание — POST правил с возвратом в список. */
export const metadata: Metadata = {
  title: 'Создать задачу — Рентли',
};

export default async function TaskCreateRoutePage({ params }: PageProps<'/properties/[id]/tasks/new'>) {
  const { id } = await params;

  return <TaskCreateScreen initialPropertyId={id} />;
}
