'use client';

import { usePathname } from 'next/navigation';
import { useSyncExternalStore, type JSX } from 'react';
import { TopNavView, mergeTopNavConfig, topNavPatchStore } from '@/shared/ui/design';
import { matchTopNavRoute } from '../model/top-nav-registry';

/** Персистентная шапка (решение владельца 23.10.2026, issue #1592 —
 * анти-дёргание аватара): единственный DOM-узел TopNavView, живёт в
 * ScreenLayout над контентом и переживает клиентские навигации —
 * <img> лого и аватара не перемонтируются, как у PersistentNavWings на
 * ПК (22.10). Контент — слияние базы реестра маршрутов (статика и
 * анатомия, работает на SSR — первый кадр холодного старта не отличается
 * от сегодняшнего) и регистраций экранов (topNavPatchStore — динамика,
 * после гидратации перебивает реестр). Вёрстка и поведение прежние:
 * мобайл — в потоке и уезжает со скроллом, планшет/ПК — закреплена
 * (компенсацию высоты держит screen-shell), материал opaque, кроссфейд
 * View Transitions общий (узел одинаков в старом/новом кадре — шов в
 * зоне шапки отсутствует).
 *
 * Маршрут без записи реестра и без регистраций не рендерит ничего —
 * как сегодня экран без TopNav не рисует header. */
export function PersistentTopNav(): JSX.Element | null {
  const pathname = usePathname();
  const base = matchTopNavRoute(pathname);
  const patches = useSyncExternalStore(
    topNavPatchStore.subscribe,
    topNavPatchStore.getSnapshot,
    topNavPatchStore.getServerSnapshot,
  );
  const { props, present } = mergeTopNavConfig(base, patches);
  if (!present) return null;
  return <TopNavView {...props} />;
}
