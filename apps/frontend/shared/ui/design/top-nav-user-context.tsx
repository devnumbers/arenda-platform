'use client';

import { createContext, useContext, type JSX, type ReactNode } from 'react';

/** Слот данных профиля для десктопного «крыла» TopNav (лого + кнопка
 * пользователя). Сам TopNav живёт в shared и не может зависеть от
 * фич (границы слоёв), поэтому источник данных — провайдер в слое
 * widgets (ScreenLayout подставляет useMe); вне провайдера — null и
 * плейсхолдер имени. pending — useMe ещё в полёте: TopNav рисует
 * скелетон вместо имени; ошибка и имя-null — терминальные состояния,
 * остаются на текстовом плейсхолдере. */
export type TopNavUser = {
  readonly name?: string;
  /** Путь стриминга фото профиля (ADR 0065, решение #1286): крыло несёт
   * фото, пока оно есть; null — фото нет или юзер не прочитан. */
  readonly photoUrl?: string | null;
  readonly pending?: boolean;
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
