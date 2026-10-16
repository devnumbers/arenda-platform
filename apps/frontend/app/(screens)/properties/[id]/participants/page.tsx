import type { Metadata } from 'next';
import {
  parsePropertyParticipantOrderParams,
  parsePropertyParticipantRoleFilterParams,
  PropertyParticipantsScreen,
} from '@/widgets/participants';

/** Экран «Участники объекта» (карта #692, тикет #700): замена легаси-
 * модалки совместного доступа — список, поиск, роли, приглашение. Чипы
 * «Имя» и «Все роли» живут в адресе (?order= и ?role=, #785) — стартовые
 * значения парсятся здесь, на сервере. */
export const metadata: Metadata = {
  title: 'Участники объекта — Рентли',
};

export default async function PropertyParticipantsRoutePage({
  params,
  searchParams,
}: PageProps<'/properties/[id]/participants'>) {
  const { id } = await params;
  const resolved = await searchParams;
  const initialOrder = parsePropertyParticipantOrderParams(resolved.order);
  const initialRoleFilter = parsePropertyParticipantRoleFilterParams(resolved.role);

  return (
    <PropertyParticipantsScreen
      propertyId={id}
      initialOrder={initialOrder}
      initialRoleFilter={initialRoleFilter}
    />
  );
}
