import type { Metadata } from 'next';
import { RentalCreateWizardScreen } from '@/widgets/rentals';

/**
 * Визард создания аренды (#530): один маршрут, четыре шага — клиентское
 * состояние в носителе сессии (карта #1052 D3): уход в ветви шага 4 —
 * «Выбрать контакт» (#807), «Создать контакт» (#509 с ?pick=rental),
 * карточка и правка контакта (#1159) — возвращает на тот же шаг с теми же
 * полями; выход из визарда («Закрыть»), перезагрузка и финал дают чистый
 * лист, без параметров в адресе.
 */

export const metadata: Metadata = {
  title: 'Создание аренды — Рентли',
};

export default async function RentalNewRoutePage({ params }: PageProps<'/properties/[id]/rentals/new'>) {
  const { id } = await params;

  return <RentalCreateWizardScreen propertyId={id} />;
}
