import type { Metadata } from 'next';
import { ContactEditScreen } from '@/widgets/contacts';

/** Правка контакта из книги (тот же экран #510; свойство меняется в форме —
 * «Выбрать объект», снятие привязки = «Общий контакт»). На едином хроме
 * экранов (#565). */
export const metadata: Metadata = {
  title: 'Изменить контакт — Рентли',
};

export default async function ContactEditRoutePage({ params }: PageProps<'/contacts/[contactId]/edit'>) {
  const { contactId } = await params;

  return <ContactEditScreen contactId={contactId} />;
}
