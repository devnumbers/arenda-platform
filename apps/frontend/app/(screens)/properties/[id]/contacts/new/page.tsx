import type { Metadata } from 'next';
import { ContactCreateScreen } from '@/widgets/contacts';

/** Экран «Создать контакт» (#509, макеты 1281:48439 / 1282:49285).
 * ?role= подставляет роль сразу (свободный текст), ?pick=rental — режим
 * ветви визарда аренды (#530): после создания возврат в визард с выбором
 * арендатора. searchParams читается только здесь, на сервере; в клиентский
 * виджет уходят готовые пропсы. Оболочка новых экранов (ScreenLayout,
 * колонка 560) — из layout группы (screens). */
export const metadata: Metadata = {
  title: 'Создать контакт — Рентли',
};

export default async function ContactNewRoutePage({
  params,
  searchParams,
}: PageProps<'/properties/[id]/contacts/new'>) {
  const { id } = await params;
  const query = await searchParams;
  const rawRole = query.role;
  const initialRole = typeof rawRole === 'string' ? rawRole.trim() : '';

  return (
    <ContactCreateScreen
      propertyId={id}
      initialRole={initialRole}
      returnToRentalWizard={query.pick === 'rental'}
    />
  );
}
