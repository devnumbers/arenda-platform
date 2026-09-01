'use client';

import { createContext, useContext, type JSX, type ReactNode } from 'react';

/** Слот данных профиля для десктопного «крыла» TopNav (лого + кнопка
 * пользователя). Сам TopNav живёт в shared и не может зависеть от
 * фич (границы слоёв), поэтому источник данных — провайдер в слое
 * widgets (ScreenLayout подставляет useMe); вне провайдера — null и
 * плейсхолдер имени. */
export type TopNavUser = {
  readonly name?: string;
};

export const TopNavUserContext = createContext<TopNavUser | null>(null);

export function TopNavUserContextProvider({
  value,
  children,
}: {
  readonly value: TopNavUser | null;
  readonly children: ReactNode;
}): JSX.Element {
  return <TopNavUserContext.Provider value={value}>{children}</TopNavUserContext.Provider>;
}

export function useTopNavUser(): TopNavUser | null {
  return useContext(TopNavUserContext);
}
