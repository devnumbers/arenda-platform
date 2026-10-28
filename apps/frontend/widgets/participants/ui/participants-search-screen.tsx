'use client';

import { useRef, useState, type JSX } from 'react';
import { useRouter } from 'next/navigation';
import { ArrowLeft } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { goBack, pushPage } from '@/shared/lib/navigation';
import {
  filterParticipantsByQuery,
  ParticipantRowButton,
} from '@/entities/participants';
import { useParticipantsList } from '@/features/participants';
import {
  ErrorCard,
  IconButton,
  PageContent,
  SearchField,
  TopNav,
} from '@/shared/ui/design';
import { ParticipantsListSkeleton } from './participants-list-skeletons';

/**
 * Поиск по разделу «Участники» (D3 #1313, пилюля хаба 3313-77128) —
 * отдельная страница с поисковой шапкой, канон книги контактов
 * (#1238): «←» в левом слоте закрывает поиск (возврат в раздел), поле
 * получает программный фокус, ввод фильтрует список клиентски — объём
 * мал (тарифные слоты), GET /participants приходит целиком без
 * пагинации, серверного ?search= в контракте #693 нет (канон
 * серверного поиска #601 — про большие ленты). Состояния — конвенция
 * #508: пустое поле — подсказка, по чему ищем; без совпадений —
 * «Участник не найден» (2010-134629). Страница за тарифным гейтом
 * (вложенный маршрут /participants/*) — на basic не открывается.
 */
export function ParticipantsSearchScreen(): JSX.Element {
  const router = useRouter();

  const [search, setSearch] = useState('');
  const searchInputRef = useRef<HTMLInputElement | null>(null);


  const participantsQuery = useParticipantsList();
  const participants = participantsQuery.data ?? [];
  const query = search.trim();
  const searching = query.length > 0;
  const visible = filterParticipantsByQuery(participants, search);

  return (
    <>
      <TopNav
        variant="search"
        leading={
          <IconButton
            icon={<ArrowLeft />}
            label="Закрыть поиск"
            onClick={() => goBack(router, ROUTES.participants)}
          />
        }
      >
        <SearchField
            focusOnMount
          ref={searchInputRef}
          aria-label="Поиск участников"
          placeholder="Найти участника"
          value={search}
          onChange={(event) => setSearch(event.target.value)}
          onClear={() => setSearch('')}
        />
      </TopNav>

      <PageContent>
        {participantsQuery.isPending ? (
          <ParticipantsListSkeleton />
        ) : participantsQuery.isError ? (
          <ErrorCard
            title="Не удалось загрузить участников"
            onRetry={() => void participantsQuery.refetch()}
            loading={participantsQuery.isFetching}
            className="mt-6"
          />
        ) : !searching ? (
          <p className="px-6 pt-10 text-center text-base leading-[18px] text-content-secondary">
            Ищите по имени или почте участника
          </p>
        ) : visible.length === 0 ? (
          <p className="px-6 pt-10 text-center text-base leading-[18px] text-content-secondary">
            Участник не найден
          </p>
        ) : (
          <div className="flex flex-col px-6">
            {visible.map((participant) => (
              <ParticipantRowButton
                key={participant.id}
                participant={participant}
                onSelect={() => pushPage(router, ROUTES.participant(participant.id))}
              />
            ))}
          </div>
        )}
      </PageContent>
    </>
  );
}
