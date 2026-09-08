import type { Metadata } from 'next';
import { ContactsOfPropertyScreen } from '@/widgets/contacts';

/** Экран «Контакты объекта» (#508): книга контактов объекта (ADR 0054)
 * с серверным поиском. Оболочка новых экранов (ScreenLayout, колонка 560)
 * — из layout группы (screens). */
export const metadata: Metadata = {
  title: 'Контакты объекта — Рентли',
};

type ContactsRoutePageProps = {
  params: Promise<{ id: string }>;
};

export default async function ContactsRoutePage({ params }: ContactsRoutePageProps) {
  const { id } = await params;

  return <ContactsOfPropertyScreen propertyId={id} />;
}
