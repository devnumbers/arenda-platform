'use client';

import { type JSX, type ReactNode, Suspense } from 'react';
import { useUnreadNotificationsCount } from '@/features/notifications';
import { ServiceWorkerRegister } from '@/shared/lib/pwa/ServiceWorkerRegister';
import { ServiceWorkerUpdater } from '@/shared/lib/pwa/ServiceWorkerUpdater';
import {
  DesktopNavPills,
  DesktopSidebar,
  PageTransitionOverlay,
  PersistentNavWings,
  PersistentNavWingsProvider,
  TabBar,
  TabBarVisibilityProvider,
} from '@/shared/ui/design';
import { RealtimeStreamProvider } from '@/features/realtime';
import { HubPrefetchProvider } from './hub-prefetch-provider';
import { NotificationStreamGate } from './notification-stream-gate';
import { PushPermissionGate } from './push-permission-gate';
import { TopNavUserProvider } from './top-nav-user-provider';

/** Оболочка новых экранов (#460) — единственная оболочка приложения
 * (карта #556: старый кабинет снесён в #568): хром собирают сами экраны
 * своим TopNav — он же единый хедер (Figma 1185:40818–19): мобайл (560 и
 * уже) — полоса в потоке страницы во всю ширину; планшет и ПК (от 561) —
 * закреплён над прокруткой, центральная часть — колонка max-560, по краям
 * лого и кнопка профиля; высоту 72 компенсирует `tablet:pt-[72px]` (хедер
 * закреплён от 561). На ПК (от 1024, решение владельца 08.09 #561) хром
 * дополняет десктопная навигация (Figma 1603:89079): сайдбар из 6 разделов
 * слева под хедером (DesktopSidebar) и плавающие пилюли
 * «Уведомления»/«Поддержка» по нижним углам (DesktopNavPills) — оба
 * компонента сами скрыты до ПК. Снизу — TabBar (футер, мобайл и планшет
 * 1023 и уже): экраны со StickyBottomBar глушат его сами через
 * TabBarVisibilityProvider/useTabBarSuppression — вместе с ним глушатся
 * и пилюли. TopNavUserProvider прокидывает имя собственника в «крыло»
 * профиля (граница shared/feature). Здесь же живёт PWA-инфраструктура
 * приложения (ADR 0032): регистрация и тихое обновление service worker
 * и PushPermissionGate — невидимый
 * стартовый гейт пушей: heal-проверка правды подписки при загрузке и
 * авто-промпт разрешения (слайс 2 #1038, вариант (c) #1548 — окно при
 * каждом заходе); системный промпт из него поднимается при каждом заходе,
 * пока разрешение `default`.
 */
/** Число непрочитанных для бейджей навигации (#747): тот же react-query
 * запрос, что у чипа ленты — SSE и refetch-on-focus обновляют кэш один
 * раз, все потребители перерисовываются. null — счёт ещё не приходил
 * (скелет: бейджи молчат). */
function useNavigationBadge(): number | null {
  const query = useUnreadNotificationsCount();
  return query.data ?? null;
}

export function ScreenLayout({ children }: { readonly children: ReactNode }): JSX.Element {
  // Подпись и адрес пункта «Объекты» — из нав-модели (решение владельца
  // #1240: direct-переход «Объект» снесён вместе с правилом #984).
  const notificationsBadge = useNavigationBadge();

  return (
    <PersistentNavWingsProvider>
      <TabBarVisibilityProvider>
        <HubPrefetchProvider>
          <TopNavUserProvider>
            <div className="screen-shell flex min-h-dvh flex-col tablet:pt-[72px]">
              <DesktopSidebar />
              {/* Постоянное крыло ПК (лого + профиль): переживает навигации,
               * аватар шапки не перемонтируется (решение 22.10.2026); экраны
               * под провайдером своё ПК-крыло гасят. Мобайл/планшет — крылья
               * самих экранов, мигание гасит реестр загруженных фото. */}
              <PersistentNavWings />
              <div className="flex min-w-0 flex-1 flex-col">
                {children}
              </div>
              <DesktopNavPills notificationsBadge={notificationsBadge ?? 0} />
              <TabBar notificationsUnread={(notificationsBadge ?? 0) > 0} />
              <ServiceWorkerRegister />
              <ServiceWorkerUpdater />
              <PushPermissionGate />
              <NotificationStreamGate />
              <RealtimeStreamProvider />
              {/* Шторка переходов под Suspense: useSearchParams внутри требует
               * границы при статическом рендере (порт alterix-new). */}
              <Suspense fallback={null}>
                <PageTransitionOverlay />
              </Suspense>
            </div>
          </TopNavUserProvider>
        </HubPrefetchProvider>
      </TabBarVisibilityProvider>
    </PersistentNavWingsProvider>
  );
}
