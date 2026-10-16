'use client';

import { useEffect, type JSX, type ReactNode } from 'react';
import { useParams, useRouter } from 'next/navigation';
import { useProperty } from '@/features/properties';
import { usePaidTariff } from '@/features/subscription';
import { propertyPermissions } from '@/entities/property';
import { sharingGateRedirect } from '@/shared/lib/paid-sections';

type PropertyParticipantsGateProps = {
  readonly children: ReactNode;
};

/** Страховка тарифного гейта шаринга (#1463, решение владельца
 * 15.10.2026): покрывает весь префикс /properties/{id}/participants… —
 * список, приглашение и страницу прав, — который держал снятый с
 * proxy-гейта паттерн (глубокие ссылки, уведомления, закладки). Владелец
 * на basic после загрузки роли и тарифа уходит на интерстишал «Совместный
 * доступ недоступен» (#1470); участник объекта пользуется страницами на
 * любом тарифе («пригласили — значит пользуется», ADR 0064). replace —
 * страховка не оставляет гейтованную страницу в истории; сам шов решения
 * — sharingGateRedirect, optimistic-умолчание тарифа — usePaidTariff. */
export function PropertyParticipantsGate({
  children,
}: PropertyParticipantsGateProps): JSX.Element {
  const router = useRouter();
  const { id } = useParams<{ id: string }>();
  const propertyQuery = useProperty(id);
  const isPaid = usePaidTariff();

  const permissions = propertyPermissions(
    propertyQuery.isSuccess ? propertyQuery.data : undefined,
  );
  const sharingRedirect = sharingGateRedirect(
    permissions.canManageMembers,
    isPaid,
  );
  useEffect(() => {
    if (sharingRedirect !== null) {
      router.replace(sharingRedirect);
    }
  }, [sharingRedirect, router]);

  return <>{children}</>;
}
