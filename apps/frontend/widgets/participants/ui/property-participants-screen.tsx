'use client';

import { useEffect, useRef, useState, type JSX } from 'react';
import { useRouter } from 'next/navigation';
import {
  ArrowDown,
  ArrowLeft,
  Block,
  Cancel,
  EyeSmall,
  FullAccessSmall,
  Kebab,
  Search,
  SmallArrowDown,
  SmallArrowRight,
  TeamAdd,
  TimeHistory,
} from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { cn } from '@/shared/lib/cn';
import { useKeyboardActivation } from '@/shared/lib/hooks/useKeyboardActivation';
import { useUrlParams } from '@/shared/lib/hooks/use-url-params';
import { goBack } from '@/shared/lib/navigation';
import { notify } from '@/shared/lib/notifications';
import { useMe } from '@/features/auth';
import { useProperty } from '@/features/properties';
import { useLeaveProperty } from '@/features/participants';
import { propertyPermissions } from '@/entities/property';
import {
  usePropertyAccessMembers,
  useRevokeAllPropertyAccessMembers,
  type PropertyAccessMemberRef,
} from '@/features/access';
import { ParticipantRowBadge, sortOrderPickerGroups } from '@/entities/participants';
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
  UserAvatar,
  skeletonRowWidths,
  StickyBottomBar,
  StatusIcon,
  TopNav,
  TopNavBackButton,
  TopNavTitle,
  type PickerMenuGroup,
} from '@/shared/ui/design';
import { LiveValue } from '@/shared/ui/live-value';
import {
  DEFAULT_PROPERTY_PARTICIPANT_ORDER,
  DEFAULT_PROPERTY_PARTICIPANT_ROLE_FILTER,
  filterPropertyParticipantsByQuery,
  filterPropertyParticipantsByRole,
  PROPERTY_PARTICIPANT_ORDER_PARAMS,
  PROPERTY_PARTICIPANT_ROLE_FILTER_PARAMS,
  propertyParticipantRows,
  ROLE_FILTER_LABELS,
  serializePropertyParticipantOrderToParams,
  serializePropertyParticipantRoleFilterToParams,
  sortPropertyParticipantsByTitle,
  type PropertyParticipantEmailIcon,
  type PropertyParticipantRoleFilter,
  type PropertyParticipantRow,
  type PropertyParticipantSortOrder,
} from '../lib/property-participants-list';
import { popParticipantPopup, useParticipantPopup } from '../lib/participant-popups';
import { resolvePropertyParticipantsError } from '../lib/property-participants-error';
import { ParticipantSuccessPopup } from './participant-success-popup';
import { PropertyAccessNotFound } from './participant-fragments';

const EMAIL_ROLE_ICONS: Record<PropertyParticipantEmailIcon, typeof EyeSmall> = {
  full: FullAccessSmall,
  eye: EyeSmall,
};

/**
 * Экран «Участники объекта» (D6 #1312, макет 3298-75434): полный состав
 * с почтами и pending-приглашениями читают все участники объекта —
 * read-only у не-владельца. Владелец первым отдельным рядом, у каждого
 * ряда иконка роли у почты (открытый замок Icon/S/FullAccess — владелец
 * и full_access, глаз — просмотр), чипы статусов: «Аккаунт не создан»
 * у pending-приглашения (инфо, лейбл #1105 в титуле) и «Превышен лимит
 * объектов» у suspended-ноги.
 *
 * Чип «Имя» — клиентская сортировка (владелец всегда первым), чип «Все
 * роли» — клиентский фильтр, поиск — иконка в шапке поверх экрана:
 * объём мал, серверного ?search= нет (прецедент #697). Выбор обоих
 * чипов живёт в адресе (?order= и ?role=, дефолты не пишутся —
 * конвенция состояния в адресе, #785).
 *
 * Действия по ролям (D6): у владельца — кебаб «Пригласить участника» /
 * «История объекта» (#840) / красное «Отозвать доступ всем» (2035-82619
 * — партия DELETE members/invitations в скоупе одного объекта) и синий
 * CTA приглашения внизу; у приглашённого (любой роли) — кебаб только с
 * «Историей объекта» и красный CTA «Покинуть объект» (D6; самовыход
 * #701: DELETE .../access/members/self с канон-подтверждением, возврат к
 * объектам). На архивном объекте приглашение скрыто, отзыв, история и
 * выход живут.
 *
 * Тап по ряду — страница участника (#698, агрегат): uuid юзера, у
 * pending — почта. Статичные ряды, без шеврона: владелец (страница
 * участника про выданные доступы, своих ног у владельца нет), своя
 * строка «Имя (Вы)» (ФИО-канон D4: имя читателя без фамилии, серый
 * суффикс — канон шита фильтров #710; #770 — своей ноги в
 * manage-скоупе читателя нет, /participants/{себя} отвечает 404),
 * читающий без manage-прав (404-политика #693) и архивный объект (нога
 * участника там может быть только архивной). Пока /me не разрешился,
 * ряды статичны тоже — кнопка, ведущая на 404, не рендерится вовсе.
 * У зрителя, как в модалке, скрыты и manage-контролы (кебаб, CTA),
 * выдача на архивный объект запрещена.
 */
export function PropertyParticipantsScreen({
  propertyId,
  initialOrder,
  initialRoleFilter,
}: {
  readonly propertyId: string;
  readonly initialOrder?: PropertyParticipantSortOrder;
  readonly initialRoleFilter?: PropertyParticipantRoleFilter;
}): JSX.Element {
  const router = useRouter();
  const { write } = useUrlParams();

  const [searchMode, setSearchMode] = useState(false);
  const [search, setSearch] = useState('');
  const [sortOrder, setSortOrder] = useState<PropertyParticipantSortOrder>(
    initialOrder ?? DEFAULT_PROPERTY_PARTICIPANT_ORDER,
  );
  const [roleFilter, setRoleFilter] = useState<PropertyParticipantRoleFilter>(
    initialRoleFilter ?? DEFAULT_PROPERTY_PARTICIPANT_ROLE_FILTER,
  );
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [leaveOpen, setLeaveOpen] = useState(false);
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
  // Свой доступ отозван/приостановлен в открытой сессии (#719) — перечитывание
  // списка отвечает 404; экран показывает not-found-канон вместо generic-ошибки.
  const membersErrorKind = membersQuery.isError
    ? resolvePropertyParticipantsError(membersQuery.error)
    : null;
  const propertyQuery = useProperty(propertyId);
  const revokeAll = useRevokeAllPropertyAccessMembers(propertyId);
  const leaveProperty = useLeaveProperty();
  const meQuery = useMe();

  const members = membersQuery.data ?? [];
  // Почта участника приходит в каждой строке контракта (решение владельца
  // Почта участника приходит в каждой строке контракта (решение владельца
  // 2026-09-20, обход #758) — обогащение не требуется. Читатель целиком —
  // источник своей строки: uuid для «(Вы)», имя для канона D4
  // («Имя (Вы)» — фамилия своей строки не пишется).
  const rows = propertyParticipantRows(members, meQuery.data);
  // Управление участниками объекта — только владелец (D8 #1304, зеркало
  // owner-only политики: у полного доступа остаются чтение состава,
  // «Покинуть объект» и «История объекта», исчезают приглашение и
  // отзыв) — и на архивном объекте (отзыв разрешён), но выдача на
  // архивный запрещена — приглашение скрыто. Центральные права объекта
  // (#703): пока объект не загружен или не загрузился, мутации
  // консервативно скрыты.
  const permissions = propertyPermissions(
    propertyQuery.isSuccess ? propertyQuery.data : undefined,
  );
  const canManage = permissions.canManageMembers;
  // «Покинуть объект» (D6) — у любого приглашённого; тоже консервативно
  // ждёт загрузки объекта.
  const canLeave = permissions.canLeave;
  const isArchived = propertyQuery.data?.status === 'archived';

  const participantsCount = rows.filter((row) => !row.isOwner).length;
  // Служебные иконки шапки — только когда есть кого искать (§7): список
  // из одного владельца и ошибка загрузки их прячут; кебаб (D6) живёт
  // при тех же условиях — у владельца с действиями управления, у
  // приглашённого только с «Историей объекта».
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

  // Чипы живут в адресе (конвенция состояния в адресе, #785): у каждого
  // свой ключ (?order= и ?role=), дефолт не пишется, чужой ключ при
  // записи не трогается — переживает перезагрузку.
  const changeOrder = (order: PropertyParticipantSortOrder): void => {
    setSortOrder(order);
    write(serializePropertyParticipantOrderToParams(order), {
      own: PROPERTY_PARTICIPANT_ORDER_PARAMS,
    });
  };

  const changeRoleFilter = (role: PropertyParticipantRoleFilter): void => {
    setRoleFilter(role);
    write(serializePropertyParticipantRoleFilterToParams(role), {
      own: PROPERTY_PARTICIPANT_ROLE_FILTER_PARAMS,
    });
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

  // Самовыход (#701): канон-подтверждение, канон-тост, возврат к списку
  // объектов — деталь чужого объекта после выхода отвечает 404.
  const confirmLeave = (): void => {
    leaveProperty.mutate(propertyId, {
      onSuccess: () => {
        setLeaveOpen(false);
        notify.scenarios.access.leftProperty();
        goBack(router, ROUTES.properties);
      },
      onError: (error) => notify.scenarios.access.leavePropertyError(error),
    });
  };

  const sortChip = (
    <PickerMenu title="Сортировать" groups={sortOrderPickerGroups('Имя', sortOrder, changeOrder)}>
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
    <PickerMenu title="Роль" groups={roleFilterPickerGroups(roleFilter, changeRoleFilter)}>
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
        // Тап ведёт на агрегат /participants/{id} (#698) только в скоупе
        // читателя — полная политика в доке экрана.
        const target =
          canManage && !isArchived && meQuery.isSuccess && !row.isOwner && !row.isMe
            ? row.participantId
            : undefined;
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
                {/* Кебаб (D6): у владельца — управление, у приглашённого
                 * только «История объекта»; состав ветвят canManage и
                 * архив (приглашение на архивный запрещено). */}
                <Menu>
                  <MenuTrigger asChild>
                    <IconButton icon={<Kebab />} label="Еще — действия со списком" />
                  </MenuTrigger>
                  <MenuContent>
                    {canManage && !isArchived && (
                      <MenuItem
                        icon={<TeamAdd className="h-6 w-6" />}
                        onSelect={() =>
                          router.push(ROUTES.propertyParticipantsInvite(propertyId))
                        }
                      >
                        Пригласить участника
                      </MenuItem>
                    )}
                    {/* История объекта (#840): вход в ленту, прибитую к
                     * этому объекту; есть и на архивном — журнал живёт,
                     * пока живёт объект. */}
                    <MenuItem
                      icon={<TimeHistory className="h-6 w-6" />}
                      onSelect={() => router.push(ROUTES.historyProperty(propertyId))}
                    >
                      История объекта
                    </MenuItem>
                    {canManage && (
                      <MenuItem
                        icon={<Block className="h-6 w-6 text-error" />}
                        className="text-error"
                        onSelect={() => setConfirmOpen(true)}
                      >
                        Отозвать доступ всем
                      </MenuItem>
                    )}
                  </MenuContent>
                </Menu>
              </span>
            ) : undefined
          }
        >
          <TopNavTitle title="Участники объекта" />
        </TopNav>
      )}

      <PageContent aboveTabBarFooter>
        {membersQuery.isPending ? (
          <>
            {/* Чипы реальные — вне фазы загрузки (§7): контент встаёт на
             * их место без сдвига. */}
            {chips}
            <PropertyParticipantsSkeleton />
          </>
        ) : membersQuery.isError ? (
          membersErrorKind === 'not_found' ? (
            // Свой доступ отозван/приостановлен в открытой сессии (#719):
            // бэк скрывает нечитаемый объект как 404 — канон в доке
            // PropertyAccessNotFound.
            <PropertyAccessNotFound />
          ) : (
            <ErrorCard
              title="Не удалось загрузить участников"
              onRetry={() => void membersQuery.refetch()}
              className="mt-6"
            />
          )
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

      {canManage && !isArchived ? (
        // CTA над видимым нижним меню (решение владельца 06.10.2026, #1166).
        <StickyBottomBar aboveTabBar>
          <Button
            className="w-full"
            onClick={() => router.push(ROUTES.propertyParticipantsInvite(propertyId))}
          >
            Пригласить участника
          </Button>
        </StickyBottomBar>
      ) : (
        canLeave && (
          // «Покинуть объект» вместо кнопки приглашения (D6, 3298-75434):
          // красный CTA у приглашённого любой роли, включая архивный
          // объект — самовыход там тоже открыт.
          <StickyBottomBar aboveTabBar>
            <Button variant="danger" className="w-full" onClick={() => setLeaveOpen(true)}>
              Покинуть объект
            </Button>
          </StickyBottomBar>
        )
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

      {/* Подтверждение самовыхода — канон #701 (2010-132970), те же тексты,
       * что у «Покинуть объект» на детали объекта. */}
      <ConfirmDialog
        open={leaveOpen}
        onOpenChange={setLeaveOpen}
        title="Уверены, что хотите покинуть объект?"
        titleClassName="text-[28px] leading-8"
        description="Вы потеряете доступ к объекту пользователя. Попросить доступ можно будет снова"
        descriptionClassName="text-base"
        confirmLabel="Покинуть"
        cancelLabel="Отмена"
        confirmVariant="danger"
        pending={leaveProperty.isPending}
        onConfirm={confirmLeave}
      />

      {/* Попап «Все участники удалены» (макет 2035-82834): канон #697 —
       * зелёная галочка 48 и текст; владелец в списке остаётся. */}
      {showAllRevoked && (
        <Modal open onOpenChange={(open) => open || setShowAllRevoked(false)}>
          <ModalContent title="Все участники удалены" titleSrOnly>
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

/** Ряд списка (макет 1980-107096, Row Button): аватар 44 — фото профиля
 * участника или заглушка BoldUser (UserAvatar, решение #1286),
 * титул (своя строка — с серым суффиксом «(Вы)», канон шита фильтров
 * #710), почта с иконкой роли, чип статуса из VM (suspended/pending);
 * шеврон — только у кликабельных рядов. */
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
      <UserAvatar photoUrl={row.photoUrl} />
      <span className="flex min-w-0 flex-1 flex-col justify-center gap-1">
        <span className="truncate text-base font-medium text-content">
          {row.title}
          {row.isMe && <span className="text-content-tertiary"> (Вы)</span>}
        </span>
        {/* Иконка роли — постоянная часть второй строки (макет ставит её
         * у почты; когда почта скрыта контрактом или обогащение не
         * принесло адрес — индикатор роли остаётся сам по себе). Смена
         * роли чужой рукой оживает каноном B (#880: вспышка без
         * кроссфейда — канон плотных списков); own-защёлки не нужно —
         * свою правку роли делают на «Правах участника», сюда читатель
         * возвращается ремонтом списка. Источник перечитывания режиму B
         * не нужен: вспышка играется самой сменой значения. */}
        <span className="flex min-w-0 items-center gap-1 text-sm text-content-secondary">
          <LiveValue valueKey={row.emailIcon} mode="flash">
            <EmailIcon className="h-4 w-4 shrink-0" aria-hidden />
          </LiveValue>
          {row.subtitle !== undefined && <span className="truncate">{row.subtitle}</span>}
        </span>
        {row.badge && (
          <span>
            <ParticipantRowBadge badge={row.badge} />
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

/** Группы пикера фильтра ролей (макет 1980-109148): одна группа из трёх
 * значений, выбор применяется сразу (канон PickerMenu). Подписи — только
 * из ROLE_FILTER_LABELS («Все роли» + канон ACCESS_ROLE_LABELS). */
function roleFilterPickerGroups(
  value: PropertyParticipantRoleFilter,
  onChange: (value: PropertyParticipantRoleFilter) => void,
): ReadonlyArray<PickerMenuGroup> {
  const keys: ReadonlyArray<PropertyParticipantRoleFilter> = [
    'all',
    'viewer',
    'full_access',
  ];
  return [
    {
      options: keys.map((key) => ({
        label: ROLE_FILTER_LABELS[key],
        selected: value === key,
        onSelect: () => onChange(key),
      })),
    },
  ];
}
