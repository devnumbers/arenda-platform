import type { Metadata } from 'next';
import {
  ContactBookScreen,
  DEFAULT_CONTACT_BOOK_ORDER,
  DEFAULT_CONTACT_BOOK_SORT,
  parseContactBookSortParams,
} from '@/widgets/contacts';
import { contactsListQuery } from '@/features/contacts';
import { ServerPrefetchBoundary } from '@/shared/api/server-prefetch';
import { serverApiClient } from '@/shared/api/server-client';

// Кабинетная страница: куки-сессия и URL-состояние — всегда динамический
// рендер; force-dynamic снимает требование Suspense-границы для
// useSearchParams при статическом экспорте (аменда #1605 — in-page
// Suspense убран ради однофлушного HTML, react/react#37014).
export const dynamic = 'force-dynamic';

/** Плоская книга контактов (глобальная страница, макеты 1726:65083/65136/
 * 85937). На едином хроме экранов (#565): оболочка — ScreenLayout группы
 * (screens), вход — «Контакты» сайдбара ПК и шита «Еще». Сортировка живёт в
 * query строки (?sort=&order=) — переживает перезагрузку. */
export const metadata: Metadata = {
  title: 'Контакты — Рентли',
  description: 'Книга контактов',
};

export default async function ContactsRoutePage({
  searchParams,
}: PageProps<'/contacts'>) {
  const resolved = await searchParams;
  const { sort, order } = parseContactBookSortParams(resolved.sort, resolved.order);

  // Дефолтная порция книги — первый ключ keyset-обхода (#600); ось
  // сортировки экрана живёт в URL, сервер префетчит дефолт «Имя ↑» —
  // срез с сортировкой из адреса клиент перечитает (канон #887:
  // дефолтный срез списков).
  return (
    <ServerPrefetchBoundary
      prefetch={(queryClient) => {
        void queryClient.prefetchInfiniteQuery(contactsListQuery({
          propertyId: null,
          search: '',
          sort: DEFAULT_CONTACT_BOOK_SORT,
          order: DEFAULT_CONTACT_BOOK_ORDER,
          transport: serverApiClient,
        }));
      }}
    >
      <ContactBookScreen initialSort={sort} initialOrder={order} />
    </ServerPrefetchBoundary>
  );
}
