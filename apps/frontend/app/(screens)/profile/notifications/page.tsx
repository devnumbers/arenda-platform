import type { Metadata } from 'next';
import { NotificationSettingsScreen } from '@/widgets/notifications';

export const metadata: Metadata = {
  title: 'Настроить уведомления — Рентли',
  description: 'Каналы уведомлений по группам событий: электронная почта и пуш-уведомления',
};

/** Экран «Настроить уведомления» (#746, карта #734): саб-экран дерева
 * профиля (каркас #568) — вход из кебаба/шестерёнки ленты /notifications
 * (#744) и строки «Уведомления» хаба профиля; заглушка #744 заменена
 * целиком. Анатомия — по макетам 1789-100250/2329-150165: ведущий «Назад»
 * (history-first, фолбэк — хаб профиля), заголовок в центре шапки. Каркас
 * подэкрана собирает сам экран (#1546): спиннер пуш-мутации живёт в правом
 * слоте шапки, а полёт — в состоянии экрана (прецедент «SubScreenShell
 * внутри компонента», §7). */
export default function ProfileNotificationsPage() {
  return <NotificationSettingsScreen />;
}
