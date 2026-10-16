'use client';

import { useEffect, type JSX, type ReactNode } from 'react';
import { useRouter } from 'next/navigation';
import { useProperty } from '@/features/properties';
import { usePaidTariff } from '@/features/subscription';
import { propertyPermissions } from '@/entities/property';
import { sharingGateRedirect } from '@/shared/lib/paid-sections';

type HistoryObjectGateProps = {
  readonly propertyId: string;
  readonly children: ReactNode;
};

/** Страховка тарифного гейта объектных поверхностей истории (#1466,
 * решение владельца 15.10.2026): лента /history осталась за proxy-гейтом,
 * а /history/properties/{id} и /history/participants/{p}/properties/{id}
 * выпущены из него — proxy роль читателя не знает, участник объекта читает
 * их на любом тарифе («пригласили — значит пользуется», бекенд-гейт
 * роль-осведомлённый #1465). Роль на объекте гейт берёт из property-запроса:
 * владелец на basic после загрузки роли и тарифа уходит на интерстишал
 * «Совместный доступ недоступен» (#1470); посторонний/отозванный читает
 * прежний privacy-404 экрана — до гейта его разводка не касается. replace —
 * страховка не оставляет гейтованную страницу в истории; сам шов решения —
 * sharingGateRedirect, optimistic-умолчание тарифа — usePaidTariff. */
export function HistoryObjectGate({
  propertyId,
  children,
}: HistoryObjectGateProps): JSX.Element {
  const router = useRouter();
  const propertyQuery = useProperty(propertyId);
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
