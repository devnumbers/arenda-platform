'use client';

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useLayoutEffect,
  useMemo,
  useState,
  type JSX,
  type ReactNode,
} from 'react';
import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { HomeMain, MenuLines, NotificationSettings } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { cn } from '@/shared/lib/cn';

/** Футер новых экранов (Figma 1185:40813): белая полоса нижней навигации
 * во всю ширину вьюпорта, закреплённая снизу. Это мобайл-хром (768 и уже,
 * на десктопе `desktop:hidden`) — десктоп живёт с одним закреплённым
 * TopNav. Табы — «Объекты», «Уведомления», «Еще» (порядок как на канве
 * 1185:40813; иконка 24 + подпись 13/15; активная #171A1C, неактивная
 * #9FA8AC). Активность — по URL (решение задачи): «Объекты» — только
 * корневой список объектов, «Уведомления» — раздел уведомлений, «Еще» —
 * все остальные страницы (любые экраны внутри объекта, профиль и т.д.).
 * Низ уважает safe-area (home indicator).
 *
 * Показывается только на экранах без нижней кнопки действия: экраны со
 * StickyBottomBar глушат футер через useTabBarSuppression, поэтому выше по
 * дереву обязателен TabBarVisibilityProvider (ставит ScreenLayout) — вне
 * его (root layout, ui-kit) футер не рендерится вовсе. */
const TABS = [
  { href: ROUTES.properties, label: 'Объекты', Icon: HomeMain },
  { href: ROUTES.profileNotifications, label: 'Уведомления', Icon: NotificationSettings },
  { href: ROUTES.profile, label: 'Еще', Icon: MenuLines },
] as const;

type TabBarSuppression = {
  /** Провайдер в дереве есть; без него TabBar считается неуместным. */
  readonly present: boolean;
  /** Число смонтированных глушителей (нижних панелей действия). */
  readonly bars: number;
  readonly acquire: () => void;
  readonly release: () => void;
};

const absentSuppression: TabBarSuppression = { present: false, bars: 0, acquire: () => {}, release: () => {} };

const TabBarSuppressionContext = createContext<TabBarSuppression>(absentSuppression);

const useIsomorphicLayoutEffect = typeof window === 'undefined' ? useEffect : useLayoutEffect;

/** Глушит TabBar, пока монтирован вызывавший компонент (канонично —
 * StickyBottomBar). Layout-эффект меняет счётчик до отрисовки кадра —
 * футер не мигает поверх нижней кнопки действия; серверу эффект не нужен. */
export function useTabBarSuppression(): void {
  const { acquire, release } = useContext(TabBarSuppressionContext);
  useIsomorphicLayoutEffect(() => {
    acquire();
    return release;
  }, [acquire, release]);
}

export function TabBarVisibilityProvider({ children }: { readonly children: ReactNode }): JSX.Element {
  const [bars, setBars] = useState(0);
  const acquire = useCallback((): void => setBars((n) => n + 1), []);
  const release = useCallback((): void => setBars((n) => n - 1), []);
  const value = useMemo<TabBarSuppression>(
    () => ({ present: true, bars, acquire, release }),
    [bars, acquire, release],
  );

  return <TabBarSuppressionContext.Provider value={value}>{children}</TabBarSuppressionContext.Provider>;
}

/** Активный таб (решение задачи): «Объекты» — только корневой список
 * объектов (сегодня он в старом кабинете, где футера нет, — маппинг задел
 * на миграцию), «Уведомления» — раздел уведомлений, «Еще» — все остальные
 * страницы. */
function activeTabIndex(pathname: string): number {
  const indexByHref = (href: string): number => TABS.findIndex((tab) => tab.href === href);

  if (
    pathname === ROUTES.profileNotifications
    || pathname.startsWith(`${ROUTES.profileNotifications}/`)
  ) {
    return indexByHref(ROUTES.profileNotifications);
  }
  if (pathname === ROUTES.properties) {
    return indexByHref(ROUTES.properties);
  }
  return indexByHref(ROUTES.profile);
}

export function TabBar(): JSX.Element | null {
  const pathname = usePathname();
  const { present, bars } = useContext(TabBarSuppressionContext);

  if (!present || bars > 0) return null;

  const activeIndex = activeTabIndex(pathname);

  return (
    <nav
      aria-label="Нижняя навигация"
      className="fixed inset-x-0 bottom-0 z-40 w-full bg-white font-sans pb-[env(safe-area-inset-bottom)] desktop:hidden"
    >
      {/* На мобайле (768 и уже) табы во всю ширину вьюпорта; колонка 560 —
       * только на десктопе, где футер всё равно скрыт (задел на будущее). */}
      <div className="mx-auto flex h-[72px] w-full desktop:max-w-[560px] items-stretch px-4">
        {TABS.map(({ href, label, Icon }, index) => {
          const active = index === activeIndex;
          return (
            <Link
              key={href}
              href={href}
              aria-current={active ? 'page' : undefined}
              className="flex min-w-0 flex-1 cursor-pointer items-stretch justify-center rounded-button outline-none focus-visible:ring-2 focus-visible:ring-primary"
            >
              {/* Цвет живёт на внутреннем span, а не на ссылке: безслойный
               * сброс `a { color: inherit }` в globals.css перебивает
               * цветовые утилиты на самой `<a>` (см. комментарий про
               * HeroUI-normalize в globals.css). */}
              <span
                className={cn(
                  'flex min-w-0 flex-1 flex-col items-center justify-center gap-1.5 rounded-button transition-colors',
                  active
                    ? 'text-content'
                    : 'text-content-tertiary hover:text-content-secondary active:text-content-secondary',
                )}
              >
                <Icon className="h-6 w-6 shrink-0" aria-hidden />
                <span className="text-xs font-medium">{label}</span>
              </span>
            </Link>
          );
        })}
      </div>
    </nav>
  );
}
