import type { JSX, ReactNode } from 'react';
import { ServiceWorkerRegister } from '@/shared/lib/pwa/ServiceWorkerRegister';
import { ServiceWorkerUpdater } from '@/shared/lib/pwa/ServiceWorkerUpdater';
import { TabBar, TabBarVisibilityProvider } from '@/shared/ui/design';
import { TopNavUserProvider } from './top-nav-user-provider';

/** Оболочка новых экранов (#460): хром собирают сами экраны своим TopNav —
 * он же единый хедер (Figma 1185:40818–19): мобайл (768 и уже) — полоса в
 * потоке страницы во всю ширину; десктоп (от 769) — закреплён над
 * прокруткой, центральная часть — колонка max-560, по краям лого и кнопка
 * профиля; высоту 72 компенсирует `desktop:pt-[72px]`. Снизу — TabBar
 * (футер, только мобайл): экраны со StickyBottomBar глушат его сами через
 * TabBarVisibilityProvider/useTabBarSuppression. TopNavUserProvider
 * прокидывает имя собственника в «крыло» профиля (граница shared/feature).
 * Sidebar/BottomNav старого кабинета здесь отсутствуют; PWA-инфраструктура
 * (регистрация и обновление service worker) общая с CabinetLayout. */
export function ScreenLayout({ children }: { readonly children: ReactNode }): JSX.Element {
  return (
    <TabBarVisibilityProvider>
      <TopNavUserProvider>
        <div className="flex min-h-screen flex-col desktop:pt-[72px]">
          {children}
          <TabBar />
          <ServiceWorkerRegister />
          <ServiceWorkerUpdater />
        </div>
      </TopNavUserProvider>
    </TabBarVisibilityProvider>
  );
}
