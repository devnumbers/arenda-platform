'use client';

import type { JSX } from 'react';
import { usePathname } from 'next/navigation';
import { getActiveNavItem } from '@/shared/config/get-active-nav-item';
import { navSectionById, secondaryNavSections } from '@/shared/config/navigation';
import { DesktopMenuButton } from './desktop-menu-button';
import { useTabBarSuppressionState } from './tab-bar';

/** Плавающие пилюли десктопа (Figma 1675:54098/54096, тикет #561):
 * «Уведомления» закреплены в левом-нижнем углу (кнопка 200), «Поддержка» —
 * в правом-нижнем (авто-ширина); обёртки p-12 держат плашку у краёв
 * вьюпорта. Разделы — secondaryNavSections нав-модели (#558), активность —
 * getActiveNavItem по их списку: «Уведомления» подсвечивается на
 * /profile/notifications*. Только десктоп ≥769 — рендерит ScreenLayout.
 * Пока на экране смонтирован StickyBottomBar, пилюли глушатся вместе с
 * TabBar (useTabBarSuppression): нижняя панель — полноширинный белый шит,
 * углы под ним не кликабельны — тот же канон «честной недоступности», что
 * у шита «Еще» (решение не фиксировано картой — «Not yet specified»,
 * предъявлено владельцу на приёмке #561). */
export function DesktopNavPills(): JSX.Element | null {
  const { present, bars } = useTabBarSuppressionState();
  const pathname = usePathname();
  const activeSectionId = getActiveNavItem(pathname, secondaryNavSections)?.id;

  if (!present || bars > 0) return null;

  return (
    <nav aria-label="Дополнительная навигация" className="hidden font-sans desktop:block">
      <div className="fixed bottom-0 left-0 z-30 p-3">
        <DesktopMenuButton
          section={navSectionById('notifications')}
          active={activeSectionId === 'notifications'}
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
