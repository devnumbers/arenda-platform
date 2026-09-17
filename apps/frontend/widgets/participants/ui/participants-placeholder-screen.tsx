import type { JSX, ReactNode } from 'react';
import { ROUTES } from '@/shared/config/routes';
import { SubScreenShell } from '@/shared/ui/design';

/**
 * Нейтральный каркас подэкрана «Совместного доступа» (карта #692):
 * живой маршрут с канонным хедером подэкрана (SubScreenShell §12, назад —
 * на хаб), контент приедет своим тикетом — #697 «Ваши участники», #701
 * «Объекты пользователей», #699 «Пригласить участника» (прецедент
 * каркасов — тарифные подэкраны до #621).
 */
export function ParticipantsPlaceholderScreen({
  title,
  children,
}: {
  readonly title: string;
  readonly children?: ReactNode;
}): JSX.Element {
  return (
    <SubScreenShell title={title} fallbackHref={ROUTES.participants}>
      {children}
    </SubScreenShell>
  );
}
