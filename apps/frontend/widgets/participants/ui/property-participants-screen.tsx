'use client';

import { useEffect, useRef, useState, type JSX } from 'react';
import { useParams, useRouter } from 'next/navigation';
import {
  ArrowDown,
  ArrowLeft,
  BoldUser,
  Cancel,
  EditSmall,
  EyeSmall,
  Kebab,
  LockSmall,
  Search,
  SmallArrowDown,
  SmallArrowRight,
} from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { cn } from '@/shared/lib/cn';
import { useKeyboardActivation } from '@/shared/lib/hooks/useKeyboardActivation';
import { useMe } from '@/features/auth';
import { useProperty } from '@/features/properties';
import { propertyPermissions } from '@/entities/property';
import {
  usePropertyAccessMembers,
  useRevokeAllPropertyAccessMembers,
  type PropertyAccessMemberRef,
} from '@/features/access';
import { ParticipantStatusBadge } from '@/entities/participants';
import { useParticipantsList } from '@/features/participants';
import {
  Button,
  ChipButton,
  ConfirmDialog,
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
  Skeleton,
  skeletonRowWidths,
  StickyBottomBar,
  StatusIcon,
  TopNav,
  TopNavBackButton,
  TopNavTitle,
  type PickerMenuGroup,
} from '@/shared/ui/design';
import {
  filterPropertyParticipantsByQuery,
  filterPropertyParticipantsByRole,
  participantEmailIndex,
  propertyParticipantRows,
  sortPropertyParticipantsByTitle,
  type PropertyParticipantEmailIcon,
  type PropertyParticipantRoleFilter,
  type PropertyParticipantRow,
  type PropertyParticipantSortOrder,
} from '../lib/property-participants-list';
import { popParticipantPopup, useParticipantPopup } from '../lib/participant-popups';
import { ParticipantSuccessPopup } from './participant-success-popup';

const EMAIL_ROLE_ICONS: Record<PropertyParticipantEmailIcon, typeof EyeSmall> = {
  owner: LockSmall,
  edit: EditSmall,
  eye: EyeSmall,
};

const ROLE_FILTER_LABELS: Record<PropertyParticipantRoleFilter, string> = {
  all: 'Все роли',
  viewer: 'Просмотр',
  full_access: 'Редактирование',
};

/**
 * Экран «Участники объекта» (карта #692, тикет #700; Figma 1980-107096):
 * замена легаси-модалки PropertySharingModal — вход из детали объекта
 * («Управление» → «Совместный доступ»); зритель видит тот же экран.
 * Данные — GET /properties/{id}/access/members (тот же контракт, что у
 * модалки): владелец отдельным первым рядом, участники с иконкой роли у
 * почты (замок/перо/глаз) и warning-чипом приостановленного слота.
 *
 * Чип «Имя» — клиентская сортировка (владелец всегда первым), чип «Все
 * роли» — клиентский фильтр (1980-109148), поиск — иконка в шапке поверх
 * экрана (1980-108531): объём мал, серверного ?search= нет (прецедент
 * #697). Кебаб (1980-139712): «Пригласить участника» и «Отозвать доступ
 * всем» (2035-82619 — партия DELETE members/invitations в скоупе одного
 * объекта; пункт «История объекта» — журнал #712, вне скоупа тикета).
 * CTA «Пригласить участника» — приглашение от объекта без выбора объектов.
 *
 * Тап по ряду — страница участника (#698, агрегат): uuid юзера, у
 * pending — почта. Ряд владельца статичный (страница участника про
 * выданные доступы, своих ног у владельца нет). Читающий без manage-прав
 * не входит в скоуп агрегата (404-политика #693), а на архивном объекте
 * нога участника может быть только архивной — в обоих случаях ряды
 * статичные, без шеврона; у зрителя, как в модалке, скрыты и
 * manage-контролы (кебаб, CTA), выдача на архивный объект запрещена.
 */
export function PropertyParticipantsScreen(): JSX.Element {
  const params = useParams<{ id: string }>();
  const propertyId = params.id;
  const router = useRouter();

  const [searchMode, setSearchMode] = useState(false);
  const [search, setSearch] = useState('');
  const [sortOrder, setSortOrder] = useState<PropertyParticipantSortOrder>('asc');
  const [roleFilter, setRoleFilter] = useState<PropertyParticipantRoleFilter>('all');
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [showAllRevoked, setShowAllRevoked] = useState(false);
  // «Участник приглашен» (макет 2035-82202): возврат из приглашения от
  // объекта — попап рендерится здесь по staged-флагу (канон #697/#699).
  const invitedPopup = useParticipantPopup() === 'invited';

  const searchInputRef = useRef<HTMLInputElement | null>(null);
  useEffect(() => {
    if (searchMode) {
      searchInputRef.current?.focus();
    }
  }, [searchMode]);

  const membersQuery = usePropertyAccessMembers(propertyId);
  const propertyQuery = useProperty(propertyId);
  const participantsQuery = useParticipantsList();
  const revokeAll = useRevokeAllPropertyAccessMembers(propertyId);
  const meQuery = useMe();

  const members = membersQuery.data ?? [];
  // Почта зарегистрированных в контракте members скрыта (только флаг
  // has_email) — добираем из кэша агрегатов /participants; собственная
  // почта и так есть в /me.
  const emailByUserId = participantEmailIndex(participantsQuery.data ?? []);
  const rows = propertyParticipantRows(members, meQuery.data?.id, {
    emailByUserId,
    currentUserEmail: meQuery.data?.email ?? undefined,
  });
  // Управление доступно владельцу и manage-участнику и на архивном объекте
  // (отзыв разрешён), но выдача на архивный запрещена — приглашение скрыто.
  // Центральные права объекта (#703): пока объект не загружен или не
  // загрузился, мутации консервативно скрыты.
  const canManage = propertyPermissions(propertyQuery.data).canManageMembers;
  const isArchived = propertyQuery.data?.status === 'archived';

  const participantsCount = rows.filter((row) => !row.isOwner).length;
  // Служебные иконки шапки — только когда есть кого искать и отзывывать
  // (§7): список из одного владельца и ошибка загрузки их прячут.
  const headerActionsVisible =
    !membersQuery.isPending && !membersQuery.isError && participantsCount > 0;
  const query = search.trim();
  const searching = searchMode && query.length > 0;
  const visible = sortPropertyParticipantsByTitle(
    filterPropertyParticipantsByRole(
      filterPropertyParticipantsByQuery(rows, search),
      roleFilter,
    ),
    sortOrder,
  );

  const closeSearch = (): void => {
    setSearchMode(false);
    setSearch('');
  };

  const confirmRevokeAll = (): void => {
    // Гард двойной защиты (канон #697): кебаб скрыт без участников, но
    // без гарда пустая партия выглядела бы успехом.
    const refs: ReadonlyArray<PropertyAccessMemberRef> = members.flatMap((member) =>
      member.isOwner || member.id === null
        ? []
        : [{ id: member.id, status: member.status }],
    );
    if (refs.length === 0) {
      setConfirmOpen(false);
      return;
    }
    revokeAll.mutate(refs, {
      onSuccess: (result) => {
        setConfirmOpen(false);
        if (result.failed === 0) {
          setShowAllRevoked(true);
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

  const roleFilterChip = (
    <PickerMenu title="Роль" groups={roleFilterPickerGroups(roleFilter, setRoleFilter)}>
      <ChipButton trailingIcon={<SmallArrowDown />}>
        {ROLE_FILTER_LABELS[roleFilter]}
      </ChipButton>
    </PickerMenu>
  );

  const chips = (
    <div className="mb-4 flex gap-2 px-6">
      {sortChip}
      {roleFilterChip}
    </div>
  );
  const rowsBlock = (
    <div className="flex flex-col px-6">
      {visible.map((row) => {
        // Тап ведёт на агрегат /participants/{id} (#698) только там, где
        // он гарантированно в скоупе читателя: manage-права и живой объект.
        const target =
          canManage && !isArchived && !row.isOwner ? row.participantId : undefined;
        return (
          <PropertyParticipantRowView
            key={row.key}
            row={row}
            onSelect={
              target === undefined
                ? undefined
                : () => router.push(ROUTES.participant(target))
            }
          />
        );
      })}
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
          leading={<TopNavBackButton fallbackHref={ROUTES.property(propertyId)} />}
          trailing={
            headerActionsVisible ? (
              <span className="flex items-center gap-1 pr-3.5">
                <IconButton
                  icon={<Search />}
                  label="Поиск участников"
                  onClick={() => setSearchMode(true)}
                />
                {canManage && (
                  <Menu>
                    <MenuTrigger asChild>
                      <IconButton icon={<Kebab />} label="Еще — действия со списком" />
                    </MenuTrigger>
                    <MenuContent>
                      {!isArchived && (
                        <MenuItem
                          onSelect={() =>
                            router.push(ROUTES.propertyParticipantsInvite(propertyId))
                          }
                        >
                          Пригласить участника
                        </MenuItem>
                      )}
                      <MenuItem onSelect={() => setConfirmOpen(true)}>
                        Отозвать доступ всем
                      </MenuItem>
                    </MenuContent>
                  </Menu>
                )}
              </span>
            ) : undefined
          }
        >
          <TopNavTitle title="Участники объекта" />
        </TopNav>
      )}

      <PageContent>
        {membersQuery.isPending ? (
          <>
            {/* Чипы реальные — вне фазы загрузки (§7): контент встаёт на
             * их место без сдвига. */}
            {chips}
            <PropertyParticipantsSkeleton />
          </>
        ) : membersQuery.isError ? (
          <ErrorCard
            title="Не удалось загрузить участников"
            onRetry={() => void membersQuery.refetch()}
            className="mt-6"
          />
        ) : searchMode && query.length === 0 ? (
          // Подсказка пустого поиска (макет 1980-108531) — вместо списка.
          <p className="px-6 pt-16 text-center text-base leading-[18px] text-content-secondary">
            Введите имя или почту участника
          </p>
        ) : searching && visible.length === 0 ? (
          <p className="px-6 pt-16 text-center text-base leading-[18px] text-content-secondary">
            Участник не найден
          </p>
        ) : searchMode ? (
          rowsBlock
        ) : (
          <>
            {chips}
            {rowsBlock}
          </>
        )}
      </PageContent>

      {canManage && !isArchived && (
        <StickyBottomBar>
          <Button
            className="w-full"
            onClick={() => router.push(ROUTES.propertyParticipantsInvite(propertyId))}
          >
            Пригласить участника
          </Button>
        </StickyBottomBar>
      )}

      <ConfirmDialog
        open={confirmOpen}
        onOpenChange={setConfirmOpen}
        title="Отозвать доступ всем пользователям к вашему объекту?"
        titleClassName="text-[28px] leading-8"
        description="Пользователи потеряют доступ к вашему объекту и будут удалены из списка участников. Пригласить их можно будет снова"
        confirmLabel="Отозвать и удалить"
        cancelLabel="Отмена"
        confirmVariant="danger"
        stacked
        pending={revokeAll.isPending}
        onConfirm={confirmRevokeAll}
      />

      {/* Попап «Все участники удалены» (макет 2035-82834): канон #697 —
       * зелёная галочка 48 и текст; владелец в списке остаётся. */}
      {showAllRevoked && (
        <Modal open onOpenChange={(open) => open || setShowAllRevoked(false)}>
          <ModalContent title="Все участники удалены" titleSrOnly className="relative">
            <IconButton
              icon={<Cancel />}
              label="Закрыть"
              onClick={() => setShowAllRevoked(false)}
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

      {invitedPopup && (
        <ParticipantSuccessPopup
          title="Участник приглашен"
          onClose={() => popParticipantPopup()}
        />
      )}
    </>
  );
}

/** Ряд списка (макет 1980-107096, Row Button): аватар 44 с BoldUser,
 * титул (у владельца — «(Вы)» из VM), почта с иконкой роли, чип
 * приостановленного слота; шеврон — только у кликабельных рядов. */
function PropertyParticipantRowView({
  row,
  onSelect,
}: {
  readonly row: PropertyParticipantRow;
  readonly onSelect?: () => void;
}): JSX.Element {
  const activatorProps = useKeyboardActivation({ onSelect });
  const EmailIcon = EMAIL_ROLE_ICONS[row.emailIcon];

  return (
    <div
      {...activatorProps}
      className={cn(
        'group/row flex w-full items-center gap-3 py-3 text-left outline-none',
        'transition-opacity focus-visible:ring-4 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-white',
        onSelect !== undefined
          ? 'cursor-pointer hover:opacity-80 active:opacity-80'
          : 'cursor-default',
      )}
    >
      <span
        aria-hidden
        className="flex h-11 w-11 shrink-0 items-center justify-center rounded-pill bg-surface-muted shadow-[0_0_0_2.5px_var(--dl-surface)]"
      >
        <BoldUser className="h-6 w-6" />
      </span>
      <span className="flex min-w-0 flex-1 flex-col justify-center gap-1">
        <span className="truncate text-base font-medium text-content">{row.title}</span>
        {/* Иконка роли — постоянная часть второй строки (макет ставит её
         * у почты; когда почта скрыта контрактом или обогащение не
         * принесло адрес — индикатор роли остаётся сам по себе). */}
        <span className="flex min-w-0 items-center gap-1 text-sm text-content-secondary">
          <EmailIcon className="h-4 w-4 shrink-0" aria-hidden />
          {row.subtitle !== undefined && <span className="truncate">{row.subtitle}</span>}
        </span>
        {row.suspended && (
          <span>
            <ParticipantStatusBadge
              badge={{ tone: 'warning', label: 'Превышен лимит объектов', withLock: true }}
            />
          </span>
        )}
      </span>
      {onSelect !== undefined && (
        <SmallArrowRight className="h-6 w-6 shrink-0 text-content-tertiary" aria-hidden />
      )}
    </div>
  );
}

/** Скелетон списка (§7): каркас ряда — аватар 44, титул 18, почта 16,
 * шеврон; паддинг строки 12px 0, как у реальной. */
function PropertyParticipantsSkeleton(): JSX.Element {
  return (
    <div aria-hidden className="flex flex-col px-6">
      {skeletonRowWidths(4).map((widths, index) => (
        <span key={index} className="flex w-full items-center gap-3 py-3">
          <Skeleton className="h-11 w-11 shrink-0 rounded-pill" />
          <span className="flex min-w-0 flex-1 flex-col gap-1">
            <Skeleton className={`h-[18px] ${widths.title}`} />
            <Skeleton className={`h-4 ${widths.subtitle}`} />
          </span>
          <Skeleton className="h-6 w-6 shrink-0" />
        </span>
      ))}
    </div>
  );
}

/** Группы пикера сортировки «Имя» — канон #697. */
function sortPickerGroups(
  order: PropertyParticipantSortOrder,
  onChange: (order: PropertyParticipantSortOrder) => void,
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

/** Группы пикера фильтра ролей (макет 1980-109148): одна группа из трёх
 * значений, выбор применяется сразу (канон PickerMenu). */
function roleFilterPickerGroups(
  value: PropertyParticipantRoleFilter,
  onChange: (value: PropertyParticipantRoleFilter) => void,
): ReadonlyArray<PickerMenuGroup> {
  return [
    {
      options: [
        { label: 'Все роли', selected: value === 'all', onSelect: () => onChange('all') },
        { label: 'Просмотр', selected: value === 'viewer', onSelect: () => onChange('viewer') },
        {
          label: 'Редактирование',
          selected: value === 'full_access',
          onSelect: () => onChange('full_access'),
        },
      ],
    },
  ];
}
