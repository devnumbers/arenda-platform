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
import { MoreSheet } from './more-sheet';
import { TabBarRow } from './tab-bar-row';

/** Футер новых экранов (Figma 1721:64793): белая полоса нижней навигации
 * во всю ширину вьюпорта, закреплённая снизу. Это хром мобайла и планшета
 * (1023 и уже, на ПК ≥1024 `desktop:hidden` — ПК живёт с сайдбаром и
 * пилюлями; ярусы владельца 08.09 #561). Табы — «Объекты», «Уведомления»,
 * «Еще»; активность — из нав-модели (getActiveMobileTab, тикет #558):
 * «Объекты» — объект и его внутренние страницы, «Уведомления» — раздел
 * уведомлений, «Еще» — всё остальное. Тап «Еще» открывает шит навигации
 * (MoreSheet, тикет #560) вместо перехода на профиль; профиль на мобайле
 * остаётся достижим через «крыло» UserButton хаб-экранов. Низ уважает
 * safe-area (home indicator).
 *
 * Показывается только на экранах без нижней кнопки действия: экраны со
 * StickyBottomBar глушат футер через useTabBarSuppression, поэтому выше по
 * дереву обязателен TabBarVisibilityProvider (ставит ScreenLayout) — вне
 * его (root layout, ui-kit) футер не рендерится вовсе. Модель «одного
 * целого» (решение владельца 22.09.2026): бар — z-50, НАД оверлеем и
 * листом шита «Еще» — не темнеет, не дублируется и остаётся живым при
 * открытом шите: «Еще» — тумблер (подсвечен активным, пока шит открыт),
 * табы-ссылки ведут на разделы и закрывают шит; лист поднимается
 * из-за бара. */
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
 * StickyBottomBar); enabled=false — экран живёт с футером (условные
 * ветви вроде создания контакта вне визарда #807). Пилюли десктопа это
 * не касается: «хром ПК постоянен» — нижняя панель на ПК сужается до
 * колонки, углы свободны (решение владельца 25.09, правка канона #561,
 * тикет #865). Layout-эффект меняет счётчик до отрисовки кадра — футер
 * не мигает поверх нижней кнопки действия; серверу эффект не нужен. */
export function useTabBarSuppression(enabled = true): void {
  const { acquire, release } = useContext(TabBarSuppressionContext);
  useIsomorphicLayoutEffect(() => {
    if (!enabled) {
      return;
    }
    acquire();
    return release;
  }, [enabled, acquire, release]);
}

/** Состояние глушения: пилюли десктопа его больше не слушают (хром ПК
 * постоянен, решение владельца 25.09 #865) и читают только present. */
export function useTabBarSuppressionState(): TabBarSuppression {
  return useContext(TabBarSuppressionContext);
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

export function TabBar({
  notificationsUnread = false,
}: {
  readonly notificationsUnread?: boolean;
} = {}): JSX.Element | null {
  const { present, bars } = useContext(TabBarSuppressionContext);
  const [moreOpen, setMoreOpen] = useState(false);

  if (!present || bars > 0) return null;

  return (
    <>
      <nav
        aria-label="Нижняя навигация"
        className="pointer-events-auto fixed inset-x-0 bottom-0 z-50 w-full bg-white pb-[max(12px,env(safe-area-inset-bottom))] font-sans desktop:hidden"
      >
        {/* Табы во всю ширину вьюпорта — футер существует только там, где
         * нет ПК-хрома (1023 и уже), кап-колонка не нужна. Фолбэк 12px в
         * safe-area — тот же, что у шита «Еще»: полоса и лист заканчиваются
         * на одной высоте (research §4). Бар живёт НАД оверлеем шита
         * (z-50 против z-40) — одна поверхность с листом, не темнеет и
         * кликабелен в любой фазе анимации. pointer-events-auto обязателен:
         * открытый шит (dismissable-layer Radix) ставит body
         * pointer-events:none — без явного значения бар «прозрачен» для
         * кликов, хотя стоит выше шита. */}
        <TabBarRow
          moreActive={moreOpen}
          moreExpanded={moreOpen}
          onMoreSelect={() => setMoreOpen((open) => !open)}
          onNavigate={() => setMoreOpen(false)}
          notificationsUnread={notificationsUnread}
        />
      </nav>
      <MoreSheet open={moreOpen} onOpenChange={setMoreOpen} />
    </>
  );
}
