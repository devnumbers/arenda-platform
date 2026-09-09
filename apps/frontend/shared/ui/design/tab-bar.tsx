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
import { cn } from '@/shared/lib/cn';
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
 * его (root layout, ui-kit) футер не рендерится вовсе. Шит «Еще» — единая
 * колонка с баром (решение владельца 2026-09-09): лист опирается на бар
 * снизу (--tab-bar-total-height), а бар на время открытого шита поднимается
 * над оверлеем и листом (z-[60]) — остаётся ярким «якорем», лист выезжает
 * из-за него; «Еще» при этом тумблер (открывает/закрывает), тапы по
 * табам-ссылкам закрывают шит перед переходом. */
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

/** Состояние глушения — для нижнего хрома, который прячется вместе с
 * TabBar, пока смонтирована нижняя панель действия (пилюли десктопа
 * #561): вне провайдера present=false (хром неуместен, рендер null),
 * bars>0 — панель перекрывает низ. */
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

export function TabBar(): JSX.Element | null {
  const { present, bars } = useContext(TabBarSuppressionContext);
  const [moreOpen, setMoreOpen] = useState(false);

  if (!present || bars > 0) return null;

  return (
    <>
      <nav
        aria-label="Нижняя навигация"
        className={cn(
          'fixed inset-x-0 bottom-0 z-40 w-full bg-white pb-[max(12px,env(safe-area-inset-bottom))] font-sans desktop:hidden',
          // Шит открыт — бар над оверлеем (z-50) и листом: яркий «якорь»,
          // лист выезжает/уезжает за ним (см. MoreSheet).
          moreOpen && 'z-[60]',
        )}
      >
        {/* Табы во всю ширину вьюпорта — футер существует только там, где
         * нет ПК-хрома (1023 и уже), кап-колонка не нужна. Фолбэк 12px в
         * safe-area — тот же, что у шита «Еще»: полоса и лист заканчиваются
         * на одной высоте (research §4). */}
        <TabBarRow
          moreExpanded={moreOpen}
          onMoreSelect={() => setMoreOpen((open) => !open)}
          onNavigate={() => setMoreOpen(false)}
        />
      </nav>
      <MoreSheet open={moreOpen} onOpenChange={setMoreOpen} />
    </>
  );
}
