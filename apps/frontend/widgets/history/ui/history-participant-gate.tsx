'use client';

import { useEffect, type JSX, type ReactNode } from 'react';
import { useRouter } from 'next/navigation';
import { usePaidTariff } from '@/features/subscription';
import { sharingGateRedirect } from '@/shared/lib/paid-sections';
import { replacePage } from '@/shared/lib/navigation';

type HistoryParticipantGateProps = {
  readonly children: ReactNode;
};

/** Тарифная разводка страницы «Действия участника» (карта #1582, зеркало
 * HistoryObjectGate #1466): поверхность владельца от и до — участники
 * читаются в скоупе владельца (D8, #1304), скоуп ленты — его объекты, на
 * basic тарифе роль-осознанный гейт бекенда отвечает 402 (ADR 0064,
 * ревизия #1465). Владельца на basic глубинная ссылка уводит на
 * интерстишал «Совместный доступ недоступен» (#1470) ещё до запроса
 * ленты; оптимистичное умолчание тарифа — usePaidTariff. replace —
 * страховка не оставляет гейтованную страницу в истории. */
export function HistoryParticipantGate({
  children,
}: HistoryParticipantGateProps): JSX.Element {
  const router = useRouter();
  const isPaid = usePaidTariff();

  const sharingRedirect = sharingGateRedirect(true, isPaid);
  useEffect(() => {
    if (sharingRedirect !== null) {
      replacePage(router, sharingRedirect);
    }
  }, [sharingRedirect, router]);

  return <>{children}</>;
}
