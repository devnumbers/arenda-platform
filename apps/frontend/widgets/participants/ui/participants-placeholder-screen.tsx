import type { JSX } from 'react';
import { ROUTES } from '@/shared/config/routes';
import { TopNav, TopNavBackButton, TopNavTitle } from '@/shared/ui/design';

/**
 * Нейтральный каркас подэкрана «Совместного доступа» (карта #692):
 * живой маршрут с канонным хедером подэкрана (назад — на хаб), контент
 * приедет своим тикетом — #697 «Ваши участники», #701 «Объекты
 * пользователей», #699 «Пригласить участника» (прецедент каркасов —
 * тарифные подэкраны до #621).
 */
export function ParticipantsPlaceholderScreen({
  title,
}: {
  readonly title: string;
}): JSX.Element {
  return (
    <TopNav leading={<TopNavBackButton fallbackHref={ROUTES.participants} />}>
      <TopNavTitle title={title} />
    </TopNav>
  );
}
