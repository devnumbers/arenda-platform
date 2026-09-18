import type { Metadata } from 'next';
import { SubScreenShell } from '@/shared/ui/design';
import { ROUTES } from '@/shared/config/routes';
import { DevicesScreen } from '@/widgets/profile';

export const metadata: Metadata = {
  title: 'Устройства — Рентли',
  description: 'Активные сессии и управление устройствами аккаунта',
};

/** Экран «Устройства» (карта #724, тикет #730; мок 1804-105061): каркас
 * подэкрана дерева профиля — «Назад» с фолбэком на хаб, шапка вне фазы
 * загрузки; список сессий рисует DevicesScreen (скелетон внутри). */
export default function DevicesPage() {
  return (
    <SubScreenShell title="Устройства" fallbackHref={ROUTES.profile}>
      <DevicesScreen />
    </SubScreenShell>
  );
}
