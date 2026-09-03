import type { Metadata } from 'next';
import { ContactEditScreen } from '@/widgets/contacts';

/** Экран «Изменить контакт» (#510, макет 1302-58933): форма создания
 * (#509) в режиме правки (предзаполнение, PATCH), удаление — с карточки.
 * Оболочка новых экранов (ScreenLayout, колонка 560) — из layout группы
 * (screens). */
export const metadata: Metadata = {
  title: 'Изменить контакт — Рентли',
};

type ContactEditRoutePageProps = {
  params: Promise<{ id: string; contactId: string }>;
};

export default async function ContactEditRoutePage({ params }: ContactEditRoutePageProps) {
  const { id, contactId } = await params;

  return <ContactEditScreen propertyId={id} contactId={contactId} />;
}
