'use client';

import { useEffect, useRef, useState, type JSX } from 'react';
import { useRouter } from 'next/navigation';
import { ArrowDown, ArrowLeft, Cancel, Kebab, Search } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import {
  filterParticipantsByQuery,
  ParticipantRowButton,
  sortParticipantsByName,
  type ParticipantSortOrder,
} from '@/entities/participants';
import {
  useParticipantsList,
  useRevokeAllParticipants,
} from '@/features/participants';
import {
  Button,
  ChipButton,
  ConfirmDialog,
  EmptyState,
  ErrorCard,
  IconButton,
  Menu,
  MenuContent,
  MenuItem,
  MenuTrigger,
  Modal,
  ModalContent,
  PageContent,
  PickerMenu,
  SearchField,
  StatusIcon,
  StickyBottomBar,
  TopNav,
  TopNavBackButton,
  TopNavTitle,
  type PickerMenuGroup,
} from '@/shared/ui/design';
import { ParticipantsListSkeleton } from './participants-list-skeletons';

/**
 * Экран «Ваши участники» (карта #692, тикет #697; Figma 2036-82971):
 * ряды участников-агрегатов (#693) с чипом агрегат-статуса, чип-сортировка
 * «Имя» (сервер приходит name ASC — направление из меню применяется
 * клиентски), кебаб «Отозвать доступ всем» и CTA «Пригласить участника»
 * в постоянной нижней панели (макет держит её во всех состояниях).
 *
 * Поиск — иконка в шапке, поверх этого же экрана (макеты 2008-84003 /
 * 2010-134629: шапка сменяется поисковой, чип сортировки прячется):
 * клиентская фильтрация — объём мал (тарифные слоты), список GET
 * /participants приходит целиком без пагинации, серверного ?search= в
 * контракте #693 нет (канон серверного поиска #601 — про большие ленты);
 * пустой результат — «Участник не найден» по центру (2010-134629).
 *
 * «Отозвать доступ всем» (макет 2008-82943) — ConfirmDialog канона #629:
 * крупный заголовок, кнопки столбиком, «Отозвать и удалить» (danger).
 * DELETE /participants/{id} уходит по каждому участнику списка; частичный
 * сбой не выглядит успехом — диалог закрывается, список перечитывается
 * (кто остался — виден), «Отозвать всех» можно повторить на остатке.
 * Успех — попап по макету 2008-84101 (прецедент продления аренды:
 * StatusIcon good 48 + зелёный текст, на мобайле закрытие свайпом/оверлеем).
 *
 * Тап по ряду — страница участника (#698); пока тикет не сделан, адрес
 * отвечает 404 осознанно.
 */
export function ParticipantsListScreen(): JSX.Element {
  const router = useRouter();

  const [searchMode, setSearchMode] = useState(false);
  const [search, setSearch] = useState('');
  const [sortOrder, setSortOrder] = useState<ParticipantSortOrder>('asc');
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [showSuccess, setShowSuccess] = useState(false);

  const searchInputRef = useRef<HTMLInputElement | null>(null);
  // Открытие поиска сразу делает поле активным (программный фокус —
  // устоявшийся a11y-паттерн поисковых шапок).
  useEffect(() => {
    if (searchMode) {
      searchInputRef.current?.focus();
    }
  }, [searchMode]);

  const participantsQuery = useParticipantsList();
  const revokeAll = useRevokeAllParticipants();

  const participants = participantsQuery.data ?? [];
  // Служебные иконки шапки — только когда данные загружены и есть что
  // искать и отзывать: пустой список (§7) и ошибка загрузки их прячут.
  const headerActionsVisible =
    !participantsQuery.isPending && !participantsQuery.isError && participants.length > 0;
  const query = search.trim();
  const searching = searchMode && query.length > 0;
  const visible = sortParticipantsByName(filterParticipantsByQuery(participants, search), sortOrder);

  const closeSearch = (): void => {
    setSearchMode(false);
    setSearch('');
  };

  const confirmRevokeAll = (): void => {
    // Гард двойной защиты: кебаб скрыт и при ошибке, и на пустом списке,
    // но без этого гарда пустая партия вернула бы {failed: 0} — ложный
    // попап успеха «Все участники удалены».
    if (participants.length === 0) {
      setConfirmOpen(false);
      return;
    }
    revokeAll.mutate(participants.map((participant) => participant.id), {
      onSuccess: (result) => {
        setConfirmOpen(false);
        if (result.failed === 0) {
          setShowSuccess(true);
        }
      },
    });
  };

  const sortChip = (
    <PickerMenu title="Сортировать" groups={sortPickerGroups(sortOrder, setSortOrder)}>
      <ChipButton
        trailingIcon={
          <ArrowDown className={sortOrder === 'desc' ? 'rotate-180' : undefined} />
        }
      >
        Имя
      </ChipButton>
    </PickerMenu>
  );

  const rows = (
    <div className="flex flex-col px-6">
      {visible.map((participant) => (
        <ParticipantRowButton
          key={participant.id}
          participant={participant}
          onSelect={() => router.push(ROUTES.participant(participant.id))}
        />
      ))}
    </div>
  );

  return (
    <>
      {searchMode ? (
        <TopNav
          variant="search"
          leading={
            <IconButton
              icon={<ArrowLeft />}
              label="Закрыть поиск"
              onClick={closeSearch}
            />
          }
        >
          <SearchField
            ref={searchInputRef}
            aria-label="Поиск участников"
            placeholder="Найти участника"
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            onClear={() => setSearch('')}
          />
        </TopNav>
      ) : (
        <TopNav
          leading={<TopNavBackButton fallbackHref={ROUTES.participants} />}
          trailing={
            headerActionsVisible ? (
              <span className="flex items-center gap-1 pr-3.5">
                <IconButton
                  icon={<Search />}
                  label="Поиск участников"
                  onClick={() => setSearchMode(true)}
                />
                <Menu>
                  <MenuTrigger asChild>
                    <IconButton icon={<Kebab />} label="Еще — действия со списком" />
                  </MenuTrigger>
                  <MenuContent>
                    <MenuItem onSelect={() => setConfirmOpen(true)}>
                      Отозвать доступ всем
                    </MenuItem>
                  </MenuContent>
                </Menu>
              </span>
            ) : undefined
          }
        >
          <TopNavTitle title="Ваши участники" />
        </TopNav>
      )}

      <PageContent>
        {participantsQuery.isPending ? (
          <>
            {/* Чип сортировки реальный — вне фазы загрузки (§7, прецедент
             * книги контактов): контент встаёт на его место без сдвига. */}
            <div className="mb-4 px-6">{sortChip}</div>
            <ParticipantsListSkeleton />
          </>
        ) : participantsQuery.isError ? (
          <ErrorCard
            title="Не удалось загрузить участников"
            onRetry={() => void participantsQuery.refetch()}
            className="mt-6"
          />
        ) : participants.length === 0 ? (
          /* Совсем пустой список: служебный чип и иконки шапки спрятаны
           * вместе с ним (§7); приглашение — CTA нижней панели. */
          <EmptyState
            imageSrc="/images/tariff/tariff-about-sharing.png"
            title="Участников пока нет"
            description="Пригласите пользователей — и дайте им доступ к выбранным объектам"
          />
        ) : searching && visible.length === 0 ? (
          <p className="px-6 pt-16 text-center text-base leading-[18px] text-content-secondary">
            Участник не найден
          </p>
        ) : searchMode ? (
          rows
        ) : (
          <>
            <div className="mb-4 px-6">{sortChip}</div>
            {rows}
          </>
        )}
      </PageContent>

      <StickyBottomBar>
        <Button className="w-full" onClick={() => router.push(ROUTES.participantsInvite)}>
          Пригласить участника
        </Button>
      </StickyBottomBar>

      <ConfirmDialog
        open={confirmOpen}
        onOpenChange={setConfirmOpen}
        title="Отозвать доступ всем пользователям к вашим объектам?"
        titleClassName="text-[28px] leading-8"
        description="Пользователи потеряют доступ ко всем вашим объектам и будут удалены из списка участников. Пригласить их можно будет снова"
        confirmLabel="Отозвать и удалить"
        cancelLabel="Отмена"
        confirmVariant="danger"
        stacked
        pending={revokeAll.isPending}
        onConfirm={confirmRevokeAll}
      />

      {/* Попап успеха (макет 2008-84101): зелёная галочка 48 и текст;
       * список под ним уже перечитан инвалидацией. Крестик — явно в углу
       * карточки на десктопе; на мобайле закрытие — свайп/оверлей (шит
       * канона), как в попапе продления аренды. */}
      {showSuccess && (
        <Modal open onOpenChange={(open) => open || setShowSuccess(false)}>
          <ModalContent title="Все участники удалены" titleSrOnly className="relative">
            <IconButton
              icon={<Cancel />}
              label="Закрыть"
              onClick={() => setShowSuccess(false)}
              className="hidden desktop:absolute desktop:right-3 desktop:top-3 desktop:block"
            />
            <div className="flex flex-col items-center gap-2">
              <StatusIcon status="good" className="h-12 w-12" />
              <p className="text-center text-base font-medium leading-[18px] text-success">
                Все участники удалены
              </p>
            </div>
          </ModalContent>
        </Modal>
      )}
    </>
  );
}

/** Группы пикера сортировки: поле («Имя» — единственное) и направление —
 * выбор применяется сразу (канон PickerMenu, как книга контактов). */
function sortPickerGroups(
  order: ParticipantSortOrder,
  onChange: (order: ParticipantSortOrder) => void,
): ReadonlyArray<PickerMenuGroup> {
  return [
    {
      options: [{ label: 'Имя', selected: true, onSelect: () => undefined }],
    },
    {
      options: [
        { label: 'Возрастание', selected: order === 'asc', onSelect: () => onChange('asc') },
        { label: 'Убывание', selected: order === 'desc', onSelect: () => onChange('desc') },
      ],
    },
  ];
}
