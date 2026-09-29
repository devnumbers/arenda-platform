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
import { ParticipantsHubSkeleton } from './participants-hub-skeletons';
import { ParticipantScreenSkeleton } from './participants-skeletons';

/**
 * Route-loading архетипы зоны «Совместный доступ» (#692, #609): fallback
 * Suspense-границ маршрутов — в окне оседания серверного префетча
 * стримится кадр с хромом экрана, а не контент без шапки (§14, «шапка
 * не прыгает»), как и обещает докстринг скелетона хаба. Контент —
 * скелетоны экранов (#696, #698); in-screen pending живых экранов
 * остаётся внутри них.
 */

/** Хаб «Совместный доступ» (#696): хаб-шапка с крыльями и свёрнутым
 * тайтлом — кадр шапки живого экрана. Иконка-приглашение (в баре и в
 * строке заголовка) — действие живого экрана: приходит вместе с данными
 * summary, в фазе загрузки не рисуется. */
export function ParticipantsHubLoading(): JSX.Element {
  return (
    <>
      <TopNav mobileWings collapse={{ title: 'Совместный доступ' }} />

      <PageContent>
        <ParticipantsHubSkeleton />
      </PageContent>
    </>
  );
}

/** Страница участника (#698): бар подэкрана «Назад + Участник» — кадр,
 * который живой экран собирает SubScreenShell («Назад» ведёт тем же
 * фоллбэком на список участников). Кебаб в покое — без onClick, фаза
 * загрузки не действует: загруженный экран подменяет его рабочим без
 * сдвига; тайтлы данных приходят с контентом. */
export function ParticipantLoading(): JSX.Element {
  return (
    <>
      <TopNav
        leading={<TopNavBackButton fallbackHref={ROUTES.participantsList} />}
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
