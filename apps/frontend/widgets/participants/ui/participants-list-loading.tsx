'use client';

import type { JSX } from 'react';
import { ArrowDown, Kebab } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import {
  ChipButton,
  HubCollapseAnchor,
  HubTitle,
  IconButton,
  PageContent,
  SearchPill,
  TopNav,
} from '@/shared/ui/design';
import { ParticipantsListSkeleton } from './participants-list-skeletons';

/**
 * Route-loading архетип списка участников (#609, тикет #1422): хром экрана
 * — вне фазы загрузки (§7): TopNav с «крыльями», хаб-ряд «Участники» с
 * кебабом в покое, пилюля поиска — нейтральный кадр живого экрана до
 * разрешения подписки (гейт чтения списка — ADR 0064 ревизия #1313); чип
 * сортировки — реальный с дефолтным лейблом «Имя» (без onClick — фаза
 * загрузки не действует; канон чипов в скелетонах, operations-loading
 * #609), тело — скелетон списка (паритет #604). Кадр рисует fallback
 * Suspense-границы страницы — после сноса сегментных loading.tsx хабов
 * (план 045 #1437) это единственный кадр холодной загрузки хаба, поэтому
 * голый скелетон без шапки в нём = регрессия «шапка прыгает» (гард —
 * e2e hub-cold-loading).
 */
export function ParticipantsListLoading(): JSX.Element {
  return (
    <>
      <TopNav
        mobileWings
        collapse={{
          title: 'Участники',
          search: { href: ROUTES.participantsSearch, label: 'Найти участника' },
          trailing: <IconButton icon={<Kebab />} label="Еще — действия со списком" />,
        }}
      />

      <PageContent>
        <HubCollapseAnchor>
          {/* Строка заголовка h-8 — зеркало живого экрана: кебаб в покое
           * справа, интерактивный экран подменяет его без сдвига. */}
          <div className="flex h-8 items-center justify-between pr-3.5">
            <HubTitle>Участники</HubTitle>
            <IconButton icon={<Kebab />} label="Еще — действия со списком" />
          </div>

          {/* Пилюля поиска — как в нейтральной фазе живого экрана (до
           * разрешения подписки она видима, §7): контент встаёт на её
           * место без сдвига. */}
          <div className="mt-4 px-6">
            <SearchPill label="Найти участника" onOpenSearch={() => {}} />
          </div>
        </HubCollapseAnchor>

        {/* Тот же ритм первого блока, что у живого экрана (макет
         * 3225-77283): чип в 24 под пилюлей и в 24 над контентом. */}
        <div className="mb-6 mt-6 px-6" data-testid="participants-list-loading">
          <ChipButton trailingIcon={<ArrowDown />}>Имя</ChipButton>
        </div>

        <ParticipantsListSkeleton />
      </PageContent>
    </>
  );
}
