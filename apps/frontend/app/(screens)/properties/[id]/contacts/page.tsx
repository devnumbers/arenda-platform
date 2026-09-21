import type { Metadata } from 'next';
import { ContactsOfPropertyScreen, parseContactListOrderParams } from '@/widgets/contacts';

/** Экран «Контакты объекта» (#508): книга контактов объекта (ADR 0054)
 * с серверным поиском. Направление сортировки живёт в адресе (?order=,
 * #785) — стартовое значение парсится здесь, на сервере. Оболочка новых
 * экранов (ScreenLayout, колонка 560) — из layout группы (screens). */
export const metadata: Metadata = {
  title: 'Контакты объекта — Рентли',
};

type ContactsRoutePageProps = {
  params: Promise<{ id: string }>;
  searchParams?: Promise<Record<string, string | string[] | undefined>>;
};

export default async function ContactsRoutePage({ params, searchParams }: ContactsRoutePageProps) {
  const { id } = await params;
  const resolved = searchParams ? await searchParams : {};
  const initialOrder = parseContactListOrderParams(resolved.order);

  return <ContactsOfPropertyScreen propertyId={id} initialOrder={initialOrder} />;
}
