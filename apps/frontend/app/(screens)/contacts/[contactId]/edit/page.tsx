import type { Metadata } from 'next';
import { ContactEditScreen } from '@/widgets/contacts';

/** Правка контакта из книги (тот же экран #510; свойство меняется в форме —
 * «Выбрать объект», снятие привязки = «Общий контакт»). На едином хроме
 * экранов (#565). */
export const metadata: Metadata = {
  title: 'Изменить контакт — Рентли',
};

type ContactEditRoutePageProps = {
  params: Promise<{ contactId: string }>;
};

export default async function ContactEditRoutePage({ params }: ContactEditRoutePageProps) {
  const { contactId } = await params;

  return <ContactEditScreen contactId={contactId} />;
}
