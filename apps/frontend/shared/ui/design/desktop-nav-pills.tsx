'use client';

import { useState, type JSX } from 'react';
import { usePathname } from 'next/navigation';
import { getActiveNavItem } from '@/shared/config/get-active-nav-item';
import { navSectionById, secondaryNavSections, supportNavSection } from '@/shared/config/navigation';
import { DesktopMenuButton } from './desktop-menu-button';
import { SupportModal } from './support-modal';
import { useTabBarSuppressionState } from './tab-bar';

/** Плавающие пилюли десктопа (Figma 1675:54098/54096, тикет #561):
 * «Уведомления» закреплены в левом-нижнем углу (кнопка 200), «Поддержка» —
 * в правом-нижнем (авто-ширина); обёртки p-12 держат плашку у краёв
 * вьюпорта. «Уведомления» — раздел из secondaryNavSections нав-модели
 * (#558), активность — getActiveNavItem: подсвечивается на
 * /profile/notifications*. «Поддержка» — действие (#766): страница /support
 * снесена, пилюля открывает модалку «Связаться с нами». Только ПК ≥1024
 * (hidden desktop:block; ярусы владельца 08.09: пилюли — часть ПК-хрома,
 * в 561–1023 планшетный хром с TabBar). Рендерит ScreenLayout.
 * Пока на экране смонтирован StickyBottomBar, пилюли глушатся вместе с
 * TabBar (useTabBarSuppression): нижняя панель — полноширинный белый шит,
 * углы под ним не кликабельны — тот же канон «честной недоступности», что
 * у шита «Еще» (решение не фиксировано картой — «Not yet specified»,
 * предъявлено владельцу на приёмке #561). */
export function DesktopNavPills(): JSX.Element | null {
  const { present, bars } = useTabBarSuppressionState();
  const pathname = usePathname();
  const [supportOpen, setSupportOpen] = useState(false);
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
          section={supportNavSection}
          onClick={() => setSupportOpen(true)}
          className="w-fit"
        />
      </div>
      <SupportModal open={supportOpen} onOpenChange={setSupportOpen} />
    </nav>
  );
}
