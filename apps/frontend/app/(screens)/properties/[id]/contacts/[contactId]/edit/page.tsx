import type { Metadata } from 'next';
import { ContactEditScreen } from '@/widgets/contacts';

/** Экран «Изменить контакт» (#510, макет 1302-58933): форма создания
 * (#509) в режиме правки (предзаполнение, PATCH), удаление — с карточки.
 * Оболочка новых экранов (ScreenLayout, колонка 560) — из layout группы
 * (screens). */
export const metadata: Metadata = {
  title: 'Изменить контакт — Рентли',
};

export default async function ContactEditRoutePage({ params }: PageProps<'/properties/[id]/contacts/[contactId]/edit'>) {
  const { id, contactId } = await params;

  return <ContactEditScreen propertyId={id} contactId={contactId} />;
}
