'use client';

import { useRef, type JSX, type ReactNode } from 'react';
import { useUnreadNotificationsCount } from '@/features/notifications';
import { ServiceWorkerRegister } from '@/shared/lib/pwa/ServiceWorkerRegister';
import { ServiceWorkerUpdater } from '@/shared/lib/pwa/ServiceWorkerUpdater';
import { PullToRefresh } from '@/shared/ui/pull-to-refresh';
import {
  DesktopNavPills,
  DesktopSidebar,
  TabBar,
  TabBarVisibilityProvider,
} from '@/shared/ui/design';
import { usePropertiesLandingHref } from '@/features/properties';
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
 * приложения (ADR 0031/0032): регистрация и тихое обновление service
 * worker, pull-to-refresh (ADR 0031 — ref делится с узлом контента:
 * жест двигает transform'ом именно его; сайдбар, пилюли и TabBar
 * `position: fixed` и остаются на месте) и PushPermissionGate —
/** Число непрочитанных для бейджей навигации (#747): тот же react-query
 * запрос, что у чипа ленты — SSE и refetch-on-focus обновляют кэш один
 * раз, все потребители перерисовываются. null — счёт ещё не приходил
 * (скелет: бейджи молчат). */
function useNavigationBadge(): number | null {
  const query = useUnreadNotificationsCount();
  return query.data ?? null;
}

export function ScreenLayout({ children }: { readonly children: ReactNode }): JSX.Element {
  // PullToRefresh drives `transform` on the content node during the gesture,
  // so the layout shares its ref with the component.
  const contentRef = useRef<HTMLDivElement>(null);
  // Лендинг таба «Объекты» (карта #583): основной объект / единственный
  // активный / список — пока список не загружен, обе поверхности ведут
  // на список (безопасный фолбэк хука).
  const propertiesHref = usePropertiesLandingHref();
  const notificationsBadge = useNavigationBadge();

  return (
    <TabBarVisibilityProvider>
      <HubPrefetchProvider>
        <TopNavUserProvider>
          {/* `screen-layout` (globals.css) на ПК выставляет --sidebar-inset —
             им TopNav центрирует свою колонку правее сайдбара; обёртка
             контента несёт тот же сдвиг паддингом, поэтому PageContent
             центрируется в пространстве правее сайдбара (тикет #865, макет
             1603-89079). Fixed-хром (сайдбар, пилюли, TabBar) паддинг не
             двигает. */}
          <div className="screen-layout flex min-h-screen flex-col tablet:pt-[72px]">
            <DesktopSidebar propertiesHref={propertiesHref} />
            <div
              className="flex min-w-0 flex-1 flex-col desktop:pl-[var(--sidebar-total)]"
              ref={contentRef}
            >
              {children}
            </div>
            <DesktopNavPills notificationsBadge={notificationsBadge ?? 0} />
            <TabBar propertiesHref={propertiesHref} notificationsUnread={(notificationsBadge ?? 0) > 0} />
            <ServiceWorkerRegister />
            <ServiceWorkerUpdater />
            <PullToRefresh contentRef={contentRef} />
            <PushPermissionGate />
            <NotificationStreamGate />
          </div>
        </TopNavUserProvider>
      </HubPrefetchProvider>
    </TabBarVisibilityProvider>
  );
}
