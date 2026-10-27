'use client';

import { createContext, useContext, type JSX, type ReactNode } from 'react';

/** Флаг «этот экран живёт под персистентной шапкой» (решение владельца
 * 23.10.2026, issue #1592): ScreenLayout оборачивает контент провайдером,
 * шим TopNav под ним не рендерит DOM, а регистрирует пропы в
 * topNavPatchStore — DOM-узел шапки один и живёт в layout, переживая
 * навигации (анти-дёргание аватара, как PersistentNavWings на ПК).
 *
 * Вне провайдера (ui-kit, логин, юнит-среда без ScreenLayout) шим рендерит
 * локальный экземпляр TopNavView — поведение прежнее: витрина дизайн-слоя
 * показывает шапку на месте, компонентные тесты видят то же дерево, что
 * до переезда. */
const PersistentTopNavHostContext = createContext(false);

export function useHostsPersistentTopNav(): boolean {
  return useContext(PersistentTopNavHostContext);
}

export function PersistentTopNavHostProvider({
  children,
}: {
  readonly children: ReactNode;
}): JSX.Element {
  return <PersistentTopNavHostContext.Provider value>{children}</PersistentTopNavHostContext.Provider>;
}
