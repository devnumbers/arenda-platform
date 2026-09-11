'use client';

import { createContext, useContext, useState, type JSX, type ReactNode } from 'react';

/**
 * Навигационный интент для prefetch-прототипа #610: нав-хром сообщает
 * «пользователь показал интерес к этому маршруту» (hover/pointerdown/
 * focus), а слушатель (HubPrefetchProvider в widgets/screen-layout)
 * решает, что прогреть. Контекст живёт в design, потому что читают его
 * сами нав-поверхности (DesktopMenuButton, TabNavLink); провайдер — выше,
 * в виджете оболочки, который собирает реестр хабов из фич (shared не
 * импортирует features). По умолчанию — no-op: вне провайдера (ui-kit,
 * будущие поверхности) навигация ведёт себя как раньше.
 */
export type NavIntentHandler = (href: string) => void;

const noopIntent: NavIntentHandler = () => {};

const NavIntentContext = createContext<NavIntentHandler>(noopIntent);

export function NavIntentProvider({
  onIntent,
  children,
}: {
  readonly onIntent: NavIntentHandler;
  readonly children: ReactNode;
}): JSX.Element {
  return <NavIntentContext.Provider value={onIntent}>{children}</NavIntentContext.Provider>;
}

export function useNavIntent(): NavIntentHandler {
  return useContext(NavIntentContext);
}

/**
 * Врезка интента в навигационный Link (DesktopMenuButton, TabNavLink):
 * первый интент (hover/pointerdown/focus) поднимает prefetch ссылки до
 * полного — у динамических маршрутов авто-prefetch качает только
 * скелет-шелл, а чанк страницы ехал бы на клике; заодно сообщает интент
 * слушателю (прогрев данных хаба). Вне провайдера prefetch остаётся
 * дефолтным, а onIntent — no-op.
 */
export function useNavIntentLink(href: string): {
  readonly prefetch: true | undefined;
  readonly onIntent: () => void;
} {
  const onIntent = useNavIntent();
  const [fullPrefetch, setFullPrefetch] = useState(false);
  const reportIntent = (): void => {
    onIntent(href);
    setFullPrefetch(true);
  };
  return { prefetch: fullPrefetch ? true : undefined, onIntent: reportIntent };
}
