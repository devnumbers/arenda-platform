'use client';

import type { JSX, ReactNode } from 'react';
import { useMe } from '@/features/auth';
import { TopNavUserContext } from '@/shared/ui/design';

/** Данные профиля для десктопного «крыла» TopNav: слой widgets имеет
 * право тянуть фичи, поэтому useMe живёт здесь, а shared/design остаётся
 * чистым (см. top-nav-user-context). Вне ScreenLayout (ui-kit) контекст
 * не установлен — TopNav покажет плейсхолдер. */
export function TopNavUserProvider({ children }: { readonly children: ReactNode }): JSX.Element {
  const { data: user } = useMe();

  return (
    <TopNavUserContext.Provider value={{ name: user?.name ?? undefined }}>
      {children}
    </TopNavUserContext.Provider>
  );
}
