'use client';

import { type JSX } from 'react';
import { usePathname } from 'next/navigation';
import { getActiveNavItem } from '@/shared/config/get-active-nav-item';
import { navSectionById, secondaryNavSections } from '@/shared/config/navigation';
import { DesktopMenuButton } from './desktop-menu-button';
import { useTabBarSuppressionState } from './tab-bar';

/** Плавающие пилюли десктопа (Figma 1675:54098/54096, тикет #561):
 * «Уведомления» закреплены в левом-нижнем углу (кнопка 200), «Поддержка» —
 * в правом-нижнем (авто-ширина); обёртки p-12 держат плашку у краёв
 * вьюпорта. Обе — разделы secondaryNavSections нав-модели (#558),
 * активность — getActiveNavItem: «Уведомления» подсвечиваются на
 * /profile/notifications*, «Поддержка» — на /support (число непрочитанных
 * — канон NumbersAlerts на пилюле, Figma 2340:50227, #747). «Поддержка»
 * с 28.10 ведёт на страницу /support (макет 40000074:80636) — модалка
 * «Связаться с нами» осталась только на шагах входа. Только ПК
 * ≥1024 (hidden desktop:block; ярусы владельца 08.09: пилюли — часть
 * ПК-хрома, в 561–1023 планшетный хром с TabBar). Рендерит ScreenLayout.
 * «Хром ПК постоянен» (решение владельца 25.09, правка канона #561,
 * тикет #865): пилюли не глушатся нижними панелями — шит панели на ПК
 * сужается до колонки 560, углы свободны. Класс .desktop-chrome — хук для
 * globals.css: при открытой полноэкранной поверхности хром поднимается
 * над ней (z-60, 25.09/#865 — возвращено планом 047), при открытой
 * модалке опускается под скрим (z-30, план 047). */
export function DesktopNavPills({ notificationsBadge = 0 }: { readonly notificationsBadge?: number } = {}): JSX.Element | null {
  const { present } = useTabBarSuppressionState();
  const pathname = usePathname();
  const activeSectionId = getActiveNavItem(pathname, secondaryNavSections)?.id;

  if (!present) return null;

  return (
    <nav
      aria-label="Дополнительная навигация"
      className="desktop-chrome relative z-30 hidden font-sans desktop:block"
    >
      <div className="fixed bottom-0 left-0 z-30 p-3">
        <DesktopMenuButton
          section={navSectionById('notifications')}
          active={activeSectionId === 'notifications'}
          badge={notificationsBadge}
        />
      </div>
      <div className="fixed bottom-0 right-0 z-30 p-3">
        <DesktopMenuButton
          section={navSectionById('support')}
          active={activeSectionId === 'support'}
          className="w-fit"
        />
      </div>
    </nav>
  );
}
