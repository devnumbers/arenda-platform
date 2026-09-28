import { Suspense } from 'react';
import type { Metadata } from 'next';
import { ContactBookLoading, ContactBookScreen, parseContactBookSortParams } from '@/widgets/contacts';
import { contactsListQuery } from '@/features/contacts';
import { ServerPrefetchBoundary } from '@/shared/api/server-prefetch';
import { serverApiClient } from '@/shared/api/server-client';

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

  return (
    <Suspense fallback={<ContactBookLoading />}>
      {/* Дефолтная порция книги — первый ключ keyset-обхода (#600); ось
       * сортировки экрана живёт в URL, сервер префетчит дефолт «Имя ↑» —
       * срез с сортировкой из адреса клиент перечитает (канон #887:
       * дефолтный срез списков). */}
      <ServerPrefetchBoundary
        prefetch={(queryClient) => {
          void queryClient.prefetchInfiniteQuery(contactsListQuery({
            propertyId: null,
            search: '',
            sort: 'name',
            order: 'asc',
            transport: serverApiClient,
          }));
        }}
      >
        <ContactBookScreen initialSort={sort} initialOrder={order} />
      </ServerPrefetchBoundary>
    </Suspense>
  );
}
