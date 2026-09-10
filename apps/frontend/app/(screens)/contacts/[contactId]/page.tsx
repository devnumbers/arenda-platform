import type { Metadata } from 'next';
import { ContactDetailScreen } from '@/widgets/contacts';

/** Карточка контакта из книги (тот же экран #510; доступ по привязке
 * карточки). На едином хроме экранов (#565). */
export const metadata: Metadata = {
  title: 'Контакт — Рентли',
};

type ContactRoutePageProps = {
  params: Promise<{ contactId: string }>;
};

export default async function ContactRoutePage({ params }: ContactRoutePageProps) {
  const { contactId } = await params;

  return <ContactDetailScreen contactId={contactId} />;
}
