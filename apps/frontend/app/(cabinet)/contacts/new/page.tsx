import type { Metadata } from 'next';
import { ContactCreateScreen } from '@/widgets/contacts';

/** Создание контакта из книги (тот же экран #509, что на странице объекта;
 * свойство выбирается в форме, по умолчанию «Общий контакт»). ?role=
 * подставляет роль сразу — паритет с созданием на объекте. searchParams
 * читается только здесь, на сервере. */
export const metadata: Metadata = {
  title: 'Создать контакт — Рентли',
};

type ContactNewRoutePageProps = {
  searchParams: Promise<Record<string, string | string[] | undefined>>;
};

export default async function ContactNewRoutePage({ searchParams }: ContactNewRoutePageProps) {
  const query = await searchParams;
  const rawRole = query.role;
  const initialRole = typeof rawRole === 'string' ? rawRole.trim() : '';

  return <ContactCreateScreen initialRole={initialRole} />;
}
