'use client';

import type { JSX } from 'react';
import { usePathname } from 'next/navigation';
import { getActiveNavItem } from '@/shared/config/get-active-nav-item';
import { mainNavSections } from '@/shared/config/navigation';
import { DesktopMenuButton } from './desktop-menu-button';

/** Десктопный сайдбар-меню ПК-версии (Figma 1675:54050, тикет #561):
 * закреплён слева под хедером — pt-72 под TopNav, px-12 и pb-12 от краёв;
 * колонка 200 из кнопок DesktopMenuButton (44, зазор 2). Разделы —
 * mainNavSections нав-модели (#558, Figma 1675:54051); активность —
 * getActiveNavItem по списку сайдбара: объект и его внутренние страницы
 * подсвечивают «Объекты», глобальные разделы — свой пункт; профиль/прочее
 * и /profile/notifications (вне шестёрки) — без подсветки. Только ПК
 * ≥1024 (hidden desktop:block; ярусы владельца 08.09: мобайл 320–560,
 * планшет 561–1023, ПК от 1024 — боковое меню есть только у ПК-версии,
 * в 561–1023 планшетный хром с TabBar). Рендерит ScreenLayout. Класс
 * .desktop-chrome — хук для globals.css: при открытой полноэкранной
 * поверхности меню поднимается над ней (z-60) и остаётся кликабельным
 * (решение владельца 25.09, доработка #865). */
export function DesktopSidebar(): JSX.Element {
  const pathname = usePathname();
  const activeSectionId = getActiveNavItem(pathname, mainNavSections)?.id;

  return (
    <nav
      aria-label="Основная навигация"
      className="desktop-chrome fixed left-0 top-0 z-30 hidden px-3 pb-3 pt-[72px] font-sans desktop:block"
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
