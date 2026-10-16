import type { JSX } from 'react';
import { Kebab } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import {
  IconButton,
  PageContent,
  TopNav,
  TopNavBackButton,
  TopNavTitle,
} from '@/shared/ui/design';
import { ParticipantScreenSkeleton } from './participants-skeletons';

/**
 * Route-loading архетип зоны «Участники» (#692, #609): fallback
 * Suspense-границы маршрута страницы участника — в окне оседания
 * серверного префетча стримится кадр с хромом экрана, а не контент без
 * шапки (§14, «шапка не прыгает»). in-screen pending живых экранов
 * остаётся внутри них.
 */

/** Страница участника (#698): бар подэкрана «Назад + Участник» — кадр,
 * который живой экран собирает SubScreenShell («Назад» ведёт тем же
 * фоллбэком на раздел «Участники»). Кебаб в покое — без onClick, фаза
 * загрузки не действует: загруженный экран подменяет его рабочим без
 * сдвига; тайтлы данных приходят с контентом. */
export function ParticipantLoading(): JSX.Element {
  return (
    <>
      <TopNav
        leading={<TopNavBackButton fallbackHref={ROUTES.participants} />}
        trailing={<IconButton icon={<Kebab />} label="Еще — действия с участником" />}
      >
        <TopNavTitle title="Участник" />
      </TopNav>

      <PageContent className="px-6">
        <ParticipantScreenSkeleton />
      </PageContent>
    </>
  );
}
