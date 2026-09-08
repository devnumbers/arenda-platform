import type { Metadata } from 'next';
import { ContactDetailScreen } from '@/widgets/contacts';

/** Экран «Контакт» (#510, макеты 1285:55112 / 1424:54725): деталка
 * карточки книги контактов (ADR 0054) с правкой и удалением. Оболочка
 * новых экранов (ScreenLayout, колонка 560) — из layout группы (screens). */
export const metadata: Metadata = {
  title: 'Контакт — Рентли',
};

type ContactRoutePageProps = {
  params: Promise<{ id: string; contactId: string }>;
};

export default async function ContactRoutePage({ params }: ContactRoutePageProps) {
  const { id, contactId } = await params;

  return <ContactDetailScreen propertyId={id} contactId={contactId} />;
}
