import type { Metadata } from 'next';
import { parseStringParam } from '@/shared/lib/parse-string-param';
import { ContactCreateScreen } from '@/widgets/contacts';

/** Создание контакта из книги (тот же экран #509, что на странице объекта;
 * свойство выбирается в форме, по умолчанию «Общий контакт»). ?role=
 * подставляет роль сразу — паритет с созданием на объекте. searchParams
 * читается только здесь, на сервере. На едином хроме экранов (#565). */
export const metadata: Metadata = {
  title: 'Создать контакт — Рентли',
};

export default async function ContactNewRoutePage({ searchParams }: PageProps<'/contacts/new'>) {
  const query = await searchParams;
  const initialRole = parseStringParam(query.role)?.trim() ?? '';

  return <ContactCreateScreen initialRole={initialRole} />;
}
