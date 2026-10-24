import type { Metadata } from 'next';
import { PageContent, TopNav, TopNavTitle } from '@/shared/ui/design';
import { meQueryOptions } from '@/features/auth';
import { ProfileHub } from '@/widgets/profile';
import { ServerPrefetchBoundary } from '@/shared/api/server-prefetch';
import { serverApiClient } from '@/shared/api/server-client';

// Кабинетная страница: куки-сессия и URL-состояние — всегда динамический
// рендер; force-dynamic снимает требование Suspense-границы для
// useSearchParams при статическом экспорте (аменда #1605 — in-page
// Suspense убран ради однофлушного HTML, react/react#37014).
export const dynamic = 'force-dynamic';

export const metadata: Metadata = {
  title: 'Профиль — Рентли',
  description: 'Страница профиля пользователя',
};

/** Хаб профиля в новом дизайне (карта #591, тикет #592; Figma 1903-38340 /
 * 1786-31288): шапка — анатомия подэкрана без «Назад» (решение владельца
 * 02.10, образец — /profile/devices): крылья только на ПК, заголовок
 * «Профиль» 16/18 в баре по центру на всех ярусах. Ниже ПК вход в профиль —
 * TabBar («Еще») и крылья других хабов, на ПК — крыло UserButton.
 * Подэкраны /profile* ссылаются на него фолбэком «Назад».
 * Серверный префетч useMe (§15, тикет #1422): граница внутри Suspense
 * поверх контента, хром маршрута вне фазы загрузки; fallback — тот же
 * скелетон, что держит хаб в pending (тот же кадр, #609). */
export default function ProfilePage() {
  return (
    <>
      <TopNav hideWingsBelowDesktop>
        <TopNavTitle title="Профиль" />
      </TopNav>
      <PageContent>
<ServerPrefetchBoundary
  prefetch={(queryClient) => {
    void queryClient.prefetchQuery(meQueryOptions(serverApiClient));
  }}
>
  <ProfileHub />
</ServerPrefetchBoundary>
      </PageContent>
    </>
  );
}
