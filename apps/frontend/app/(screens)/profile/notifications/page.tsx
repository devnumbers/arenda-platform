import type { Metadata } from 'next';
import { SubScreenShell } from '@/shared/ui/design';
import { ROUTES } from '@/shared/config/routes';
import { NotificationSettingsScreen } from '@/widgets/notifications';

export const metadata: Metadata = {
  title: 'Настроить уведомления — Рентли',
  description: 'Каналы уведомлений по группам событий: электронная почта и пуш-уведомления',
};

/** Экран «Настроить уведомления» (#746, карта #734): саб-экран дерева
 * профиля (каркас #568) — вход из кебаба/шестерёнки ленты /notifications
 * (#744) и строки «Уведомления» хаба профиля; заглушка #744 заменена
 * целиком. Анатомия — по макетам 1789-100250/2329-150165: ведущий «Назад»
 * (history-first, фолбэк — хаб профиля), заголовок в центре шапки. */
export default function ProfileNotificationsPage() {
  return (
    <SubScreenShell title="Настроить уведомления" fallbackHref={ROUTES.profile}>
      <NotificationSettingsScreen />
    </SubScreenShell>
  );
}
