'use client';

import type { JSX } from 'react';
import { usePathname } from 'next/navigation';
import { getActiveNavItem } from '@/shared/config/get-active-nav-item';
import { mainNavSections } from '@/shared/config/navigation';
import { DesktopMenuButton } from './desktop-menu-button';

/** Десктопный сайдбар-меню (Figma 1675:54050, тикет #561): закреплён слева
 * под хедером — pt-72 под TopNav, px-12 и pb-12 от краёв; колонка 200 из
 * кнопок DesktopMenuButton (44, зазор 2). Разделы — mainNavSections
 * нав-модели (#558, Figma 1675:54051); активность — getActiveNavItem по
 * списку сайдбара: объект и его внутренние страницы подсвечивают
 * «Объекты», глобальные разделы — свой пункт; профиль/прочее и
 * /profile/notifications (вне шестёрки) — без подсветки. Только десктоп
 * ≥769 (hidden desktop:block) — рендерит ScreenLayout; на мобайле/планшете
 * хром — TabBar со шитом «Еще». */
export function DesktopSidebar(): JSX.Element {
  const pathname = usePathname();
  const activeSectionId = getActiveNavItem(pathname, mainNavSections)?.id;

  return (
    <nav
      aria-label="Основная навигация"
      className="fixed left-0 top-0 z-30 hidden px-3 pb-3 pt-[72px] font-sans desktop:block"
    >
      <div className="flex w-[200px] flex-col gap-0.5">
        {mainNavSections.map((section) => (
          <DesktopMenuButton
            key={section.id}
            section={section}
            active={activeSectionId === section.id}
          />
        ))}
      </div>
    </nav>
  );
}
