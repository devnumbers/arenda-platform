import type { Metadata } from 'next';
import { ContactBookScreen, parseContactBookSortParams } from '@/widgets/contacts';

/** Плоская книга контактов (глобальная страница, макеты 1726:65083/65136/
 * 85937). Оболочка кабинета (лого-хедер, TabBar/сайдбар) — из layout
 * группы (cabinet); точка входа — сайдбар десктопа. Сортировка живёт в
 * query строки (?sort=&order=) — переживает перезагрузку. */
export const metadata: Metadata = {
  title: 'Контакты — Рентли',
  description: 'Книга контактов',
};

type ContactsRoutePageProps = {
  searchParams?: Promise<Record<string, string | string[] | undefined>>;
};

export default async function ContactsRoutePage({ searchParams }: ContactsRoutePageProps) {
  const resolved = searchParams ? await searchParams : {};
  const { sort, order } = parseContactBookSortParams(resolved.sort, resolved.order);

  return <ContactBookScreen initialSort={sort} initialOrder={order} />;
}
