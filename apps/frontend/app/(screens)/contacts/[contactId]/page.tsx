import { Suspense } from 'react';
import type { Metadata } from 'next';
import type { QueryClient } from '@tanstack/react-query';
import { ContactDetailLoading, ContactDetailScreen } from '@/widgets/contacts';
import { contactDetailQueryOptions } from '@/features/contacts';
import { propertyDetailQueryOptions } from '@/features/properties';
import type { Contact } from '@/entities/contact';
import { contactKeys } from '@/shared/api/query-keys';
import { ServerPrefetchBoundary } from '@/shared/api/server-prefetch';
import { serverApiClient } from '@/shared/api/server-client';

/** Карточка контакта из книги (тот же экран #510; доступ по привязке
 * карточки). На едином хроме экранов (#565). */
export const metadata: Metadata = {
  title: 'Контакт — Рентли',
};

/**
 * Серверная раскладка первого кадра (#887): карточка — гейт; объект
 * привязки — после её успеха (у экрана тот же гард по contact.propertyId),
 * глобальная страница без среза объекта своей детали объекта не читает.
 */
async function prefetchContactScreen(
  queryClient: QueryClient,
  contactId: string,
): Promise<void> {
  await queryClient.prefetchQuery(contactDetailQueryOptions({ contactId, transport: serverApiClient }));
  const contact = queryClient.getQueryData<Contact>(contactKeys.detail(contactId));
  if (contact === undefined || contact.propertyId === undefined) {
    return;
  }
  void queryClient.prefetchQuery(propertyDetailQueryOptions({ id: contact.propertyId, transport: serverApiClient }));
}

export default async function ContactRoutePage({ params }: PageProps<'/contacts/[contactId]'>) {
  const { contactId } = await params;

  return (
    <Suspense fallback={<ContactDetailLoading />}>
      <ServerPrefetchBoundary prefetch={(queryClient) => prefetchContactScreen(queryClient, contactId)}>
        <ContactDetailScreen contactId={contactId} />
      </ServerPrefetchBoundary>
    </Suspense>
  );
}
