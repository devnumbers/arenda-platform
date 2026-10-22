'use client';

import { createContext, useContext, type JSX, type ReactNode } from 'react';
import Link from 'next/link';
import { ROUTES } from '@/shared/config/routes';
import { HeaderLogo } from './header-logo';
import { UserButton } from './user-button';
import { useTopNavUser } from './top-nav-user-context';
import { topNavWingUser } from './top-nav-wings';

/** Постоянное «крыло» навигации ПК (лого + ссылка профиля; решение
 * владельца 22.10.2026 — анти-мигание аватара): живёт в ScreenLayout,
 * переживает клиентскую навигацию — <img> аватара не перемонтируется,
 * инвариант анти-мигания §10 выполняется буквально. Место — тот же слот,
 * где крылья рисовал TopNav: шапка экрана на ПК закреплена (tablet:fixed
 * top-0), слой повторяет координаты (fixed inset-x-0 top-0 h-72).
 *
 * Ярусы: слой виден только на ПК (hidden desktop:flex); мобайл и планшет
 * остаются на крыльях самого TopNav — там шапка в потоке/пер-страничная,
 * мигание гасит реестр загруженных фото (photo-session-cache). Крыло
 * страницы на ПК гасится TopNav'ом через useHasPersistentNavWings —
 * дубля нет.
 *
 * Слои: z-45 — над шапками страниц (z-40: центральный слой бара
 * w-full глотал клики по краям, решение 02.10), под шитами/поверхностями
 * (z-50). pointer-events-none на контейнере, auto на крыльях — полоса
 * не перекрывает центральный слой бара. Полноэкранная поверхность несёт
 * собственное крыло (overlay-канон #865) — постоянный слой прячется
 * правилом body:has(.fullscreen-surface) в globals.css (по DOM, без
 * провайдеров, как .desktop-chrome). */
const PersistentNavWingsContext = createContext(false);

/** Экраны под ScreenLayout: собственное крыло ПК не рисовать. Вне
 * провайдера (ui-kit, логин) — false, поведение прежнее. */
export function useHasPersistentNavWings(): boolean {
  return useContext(PersistentNavWingsContext);
}

export function PersistentNavWingsProvider({
  children,
}: {
  readonly children: ReactNode;
}): JSX.Element {
  return <PersistentNavWingsContext.Provider value>{children}</PersistentNavWingsContext.Provider>;
}

export function PersistentNavWings(): JSX.Element {
  const user = useTopNavUser();
  const { wingName, pending } = topNavWingUser(user);
  return (
    <div className="persistent-nav-wing pointer-events-none fixed inset-x-0 top-0 z-[45] hidden h-[72px] desktop:flex">
      <Link
        href={ROUTES.properties}
        aria-label="Объекты"
        className="pointer-events-auto absolute left-0 top-0 flex h-[72px] items-center pl-[22px] outline-none focus-visible:rounded-pill focus-visible:ring-2 focus-visible:ring-primary"
      >
        <HeaderLogo />
      </Link>
      <div className="pointer-events-auto absolute right-0 top-0 flex h-[72px] items-center">
        <UserButton name={wingName} photoUrl={user?.photoUrl ?? null} pending={pending} />
      </div>
    </div>
  );
}
