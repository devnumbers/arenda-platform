import type { Metadata } from 'next';
import { EmptyState, HubCollapseAnchor, HubTitle, PageContent, TopNav } from '@/shared/ui/design';

export const metadata: Metadata = {
  title: 'Уведомления — Рентли',
  description: 'Уведомления и их настройки',
};

/** Страница раздела «Уведомления» на едином хроме (карта #556, тикет
 * #566): пункт вторичной навигации — средний таб TabBar мобайла и пилюля
 * ПК (#560/#561), поэтому хаб-анатомия как у «Поддержки» (#567): TopNav с
 * крыльями и на мобайле, заголовок раздела 28.
 *
 * Промежуточная заглушка карты #734: прежний экран per-channel настроек
 * снят вместе со своим контрактом (решение #738, ADR 0056) — новый экран
 * «Настроить уведомления» заменит страницу целиком (#746), лента —
 * отдельный маршрут /notifications (#744). Канон заглушки — как у
 * «Участников» (#559): канонный EmptyState с универсальным пустым лого. */
export default function NotificationsPage() {
  return (
    <>
      <TopNav mobileWings collapse={{ title: 'Уведомления' }} />
      <PageContent>
        <HubCollapseAnchor>
          <HubTitle>Уведомления</HubTitle>
        </HubCollapseAnchor>
        <EmptyState
          className="mt-6"
          imageSrc="/images/empty-logo.png"
          title="Уведомления появятся здесь"
          description="Лента уведомлений и их настройки появятся в этом разделе"
        />
      </PageContent>
    </>
  );
}
