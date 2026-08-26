import type { JSX, ReactNode } from 'react';
import { ServiceWorkerRegister } from '@/shared/lib/pwa/ServiceWorkerRegister';
import { ServiceWorkerUpdater } from '@/shared/lib/pwa/ServiceWorkerUpdater';
import { GlobalHeader } from './global-header';

/** Оболочка новых экранов (#460): десктоп — глобальный top-header
 * (лого + кнопка профиля) во всю ширину вьюпорта и контент в
 * центрированной колонке max-560 (её приносит PageContent экрана вместе
 * с локальным TopNav); мобайл — только TopNav самого экрана.
 * Sidebar/BottomNav старого кабинета здесь отсутствуют; PWA-инфраструктура
 * (регистрация и обновление service worker) общая с CabinetLayout. */
export function ScreenLayout({ children }: { readonly children: ReactNode }): JSX.Element {
  return (
    <div className="flex min-h-screen flex-col">
      <GlobalHeader />
      {children}
      <ServiceWorkerRegister />
      <ServiceWorkerUpdater />
    </div>
  );
}
