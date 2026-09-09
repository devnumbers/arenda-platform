import type { Metadata } from 'next';
import { HubTitle, PageContent, TopNav } from '@/shared/ui/design';
import { NotificationSettings } from '@/widgets/profile';

export const metadata: Metadata = {
  title: 'Уведомления — Рентли',
  description: 'Управление настройками уведомлений',
};

/** Страница раздела «Уведомления» на едином хроме (карта #556, тикет
 * #566): пункт вторичной навигации — средний таб TabBar мобайла и пилюля
 * ПК (#560/#561), поэтому хаб-анатомия как у «Поддержки» (#567): TopNav с
 * крыльями и на мобайле, заголовок раздела 28; активность подчёркивается
 * нав-моделью #558 без правок. Состав прежний (настройки каналов), лента
 * уведомлений и её компоненты из Figma — отдельное усилие. */
export default function NotificationsPage() {
  return (
    <>
      <TopNav mobileWings />
      <PageContent>
        <HubTitle>Уведомления</HubTitle>
        <NotificationSettings />
      </PageContent>
    </>
  );
}
