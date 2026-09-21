'use client';

import { useState, type JSX } from 'react';
import { ArrowDown, BoldUser, Edit, Exit, EyeSmall, Kebab } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { ParticipantRowBadge } from '@/entities/participants';
import { useLeaveAllProperties, useLeaveProperty } from '@/features/participants';
import { useProperties, useProperty } from '@/features/properties';
import {
  ConfirmDialog,
  ErrorCard,
  IconButton,
  Menu,
  MenuContent,
  MenuItem,
  MenuTrigger,
  Modal,
  ModalContent,
  PickerMenu,
  SubScreenShell,
  ChipButton,
  type PickerMenuGroup,
} from '@/shared/ui/design';
import {
  sortUserPropertyRows,
  userPropertyRows,
  type ParticipantsPropertySortOrder,
  type UserPropertyBadge,
  type UserPropertyRow,
} from '../lib/participants-properties-list';
import { ObjectAvatarGlyph } from './participant-fragments';
import { ParticipantsListSkeleton } from './participants-list-skeletons';
import { ParticipantSuccessPopup } from './participant-success-popup';

/**
 * Экран «Объекты пользователей» (карта #692, тикет #701; Figma 2010-132145):
 * чужие объекты читающего — строки GET /properties с access.role ≠ owner
 * (список уже несёт и свои, и чужие; suspended-доступы скрыты сервером —
 * их показ уходит в блюр-карточки #702). Бейдж роли («Редактирование»/
 * «Просмотр») — канон participantLegBadge #698; чип-сортировка «Название» —
 * клиентская (объём мал — тарифные слоты, канон #697).
 *
 * Кебаб карточки открывает шит «Действия с объектом» (2010-132581/133849):
 * карточка объекта, ряд владельца, статус доступа и «Покинуть объект» —
 * ConfirmDialog 2010-132970 (кнопки в ряд) → DELETE members/self → попап
 * «Вы покинули объект» (2010-133204). Кебаб шапки — «Покинуть все объекты»
 * (2010-133435) → ConfirmDialog 2010-133563 (столбиком) → DELETE по каждому
 * → попап 2010-133727; частичный сбой попапом успеха не считается (канон
 * «Отозвать всех» #697). Имя владельца — из деталей GET /properties/{id}
 * (в списке его нет); почты владельца в контракте нет сознательно —
 * privacy-канон owner_name «never email» (спека PropertyAccessContext).
 *
 * Пустой список — «Вас не пригласили в объекты» по центру без картинки
 * и чипа (2010-133784); служебные иконки шапки спрятаны (§7).
 */
export function ParticipantsPropertiesScreen(): JSX.Element {
  const propertiesQuery = useProperties();
  const leave = useLeaveProperty();
  const leaveAll = useLeaveAllProperties();

  const [sortOrder, setSortOrder] = useState<ParticipantsPropertySortOrder>('asc');
  const [sheetPropertyId, setSheetPropertyId] = useState<string | null>(null);
  // Цель одиночного выхода: шит к этому моменту уже закрыт (макет 2010-132970
  // показывает диалог поверх списка).
  const [pendingLeaveId, setPendingLeaveId] = useState<string | null>(null);
  const [confirmSingleOpen, setConfirmSingleOpen] = useState(false);
  const [confirmAllOpen, setConfirmAllOpen] = useState(false);
  const [leftPopup, setLeftPopup] = useState<'single' | 'all' | null>(null);

  const rows = sortUserPropertyRows(userPropertyRows(propertiesQuery.data ?? []), sortOrder);

  // Служебные иконки шапки — только когда данные загружены и есть что
  // покидать: пустой список (§7) и ошибка загрузки их прячут.
  const headerActionsVisible =
    !propertiesQuery.isPending && !propertiesQuery.isError && rows.length > 0;

  const sheetRow = rows.find((row) => row.id === sheetPropertyId) ?? null;
  // Имя владельца для шита: в списке его нет — добирается деталями при
  // открытии (useProperty гейтится пустым id).
  const detailQuery = useProperty(sheetPropertyId ?? '');
  const ownerName = detailQuery.data?.access?.ownerName;
  const ownerEmail = detailQuery.data?.access?.ownerEmail;

  const requestLeave = (propertyId: string): void => {
    setSheetPropertyId(null);
    setPendingLeaveId(propertyId);
    setConfirmSingleOpen(true);
  };

  const confirmLeaveSingle = (): void => {
    if (pendingLeaveId === null) {
      return;
    }
    leave.mutate(pendingLeaveId, {
      onSuccess: () => {
        setConfirmSingleOpen(false);
        setPendingLeaveId(null);
        setLeftPopup('single');
      },
    });
  };

  const confirmLeaveAll = (): void => {
    if (rows.length === 0) {
      setConfirmAllOpen(false);
      return;
    }
    leaveAll.mutate(
      rows.map((row) => row.id),
      {
        onSuccess: (result) => {
          setConfirmAllOpen(false);
          if (result.failed === 0) {
            setLeftPopup('all');
          }
        },
      },
    );
  };

  const sortChip = (
    <PickerMenu title="Сортировать" groups={sortPickerGroups(sortOrder, setSortOrder)}>
      <ChipButton
        trailingIcon={
          <ArrowDown className={sortOrder === 'desc' ? 'rotate-180' : undefined} />
        }
      >
        Название
      </ChipButton>
    </PickerMenu>
  );

  return (
    <>
      <SubScreenShell
        title="Доступные объекты"
        fallbackHref={ROUTES.participants}
        trailing={
          headerActionsVisible ? (
            <span className="flex items-center pr-3.5">
              <Menu>
                <MenuTrigger asChild>
                  <IconButton icon={<Kebab />} label="Еще — действия со списком" />
                </MenuTrigger>
                <MenuContent>
                  <MenuItem
                    icon={<Exit className="h-6 w-6 text-error" />}
                    className="text-error"
                    onSelect={() => setConfirmAllOpen(true)}
                  >
                    Покинуть все объекты
                  </MenuItem>
                </MenuContent>
              </Menu>
            </span>
          ) : undefined
        }
      >
        {propertiesQuery.isPending ? (
          <>
            {/* Чип сортировки реальный — вне фазы загрузки (§7, прецедент
             * «Ваших участников» #697): контент встаёт на его место без сдвига. */}
            <div className="mb-4">{sortChip}</div>
            <ParticipantsListSkeleton />
          </>
        ) : propertiesQuery.isError ? (
          <ErrorCard
            title="Не удалось загрузить объекты"
            onRetry={() => void propertiesQuery.refetch()}
            className="mt-6"
          />
        ) : rows.length === 0 ? (
          /* Совсем пустой список: чип и иконки шапки спрятаны вместе с ним
           * (§7); по макету 2010-133784 — текст без картинки и CTA. */
          <p className="pt-16 text-center text-base leading-[18px] text-content-secondary">
            Вас не пригласили в объекты
          </p>
        ) : (
          <>
            <div className="mb-4">{sortChip}</div>
            <div className="flex flex-col">
              {rows.map((row) => (
                <UserPropertyRowItem
                  key={row.id}
                  row={row}
                  onActions={() => setSheetPropertyId(row.id)}
                />
              ))}
            </div>
          </>
        )}
      </SubScreenShell>

      {/* Шит «Действия с объектом» (2010-133846): карточка объекта, ряд
       * владельца, статус доступа и красный выход. Имя владельца приезжает
       * деталями; пока не resolution'илось — ряд не рисуется. */}
      <Modal
        open={sheetRow !== null}
        onOpenChange={(open) => {
          if (!open) {
            setSheetPropertyId(null);
          }
        }}
      >
        <ModalContent title="Действия с объектом">
          {sheetRow !== null && (
            <div className="flex flex-col">
              <div className="flex flex-col gap-6">
                <ObjectCard
                  photoUrl={sheetRow.photoUrl}
                  title={sheetRow.title}
                  subtitle={sheetRow.address}
                  badge={sheetRow.badge}
                />
                {ownerName !== undefined && (
                  <ObjectCard
                    glyph={<BoldUser className="h-6 w-6 text-content-tertiary" />}
                    title={ownerName}
                    subtitle={ownerEmail}
                    badge={ownerBadge}
                  />
                )}
              </div>
              <div className="mt-6 flex h-14 w-full rounded-2xl bg-surface-muted p-0.5">
                <div className="flex h-full w-full items-center justify-center gap-2 rounded-[14px] bg-surface shadow-[0_2px_8px_rgba(0,0,0,0.16)]">
                  {sheetRow.role === 'full_access' ? (
                    <>
                      <span className="text-sm font-medium leading-4 text-content">
                        Вам доступно редактирование
                      </span>
                      <Edit className="h-6 w-6 shrink-0 text-content" aria-hidden />
                    </>
                  ) : (
                    <>
                      <span className="text-sm font-medium leading-4 text-content">
                        Вам доступен просмотр
                      </span>
                      <EyeSmall className="h-6 w-6 shrink-0 text-content" aria-hidden />
                    </>
                  )}
                </div>
              </div>
              {/* Отступ 24 на обёртке-div: unlayered preflight
               * button{margin:0} гасит margin-утилиты на голой кнопке
               * (урок #730/#753, приёмка #757). */}
              <div className="mt-6">
                <button
                  type="button"
                  onClick={() => requestLeave(sheetRow.id)}
                  className="flex cursor-pointer items-center gap-4 text-left outline-none transition-opacity hover:opacity-80 focus-visible:ring-4 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-surface active:opacity-80"
                >
                  <Exit className="h-6 w-6 shrink-0 text-error" aria-hidden />
                  <span className="text-base font-medium leading-[18px] text-error">
                    Покинуть объект
                  </span>
                </button>
              </div>
            </div>
          )}
        </ModalContent>
      </Modal>

      {/* Покинуть один объект (2010-132970): кнопки в ряд — «Отмена» слева,
       * красное «Покинуть» справа. */}
      <ConfirmDialog
        open={confirmSingleOpen}
        onOpenChange={setConfirmSingleOpen}
        title="Уверены, что хотите покинуть объект?"
        titleClassName="text-[28px] leading-8"
        description="Вы потеряете доступ к объекту пользователя. Попросить доступ можно будет снова"
        descriptionClassName="text-base"
        confirmLabel="Покинуть"
        cancelLabel="Отмена"
        confirmVariant="danger"
        pending={leave.isPending}
        onConfirm={confirmLeaveSingle}
      />

      {/* Покинуть все объекты (2010-133563): столбиком, подтверждение
       * сверху. Частичный сбой партии закрывает диалог без попапа —
       * список перечитан, повтор возможен на остатке (канон #697). */}
      <ConfirmDialog
        open={confirmAllOpen}
        onOpenChange={setConfirmAllOpen}
        title="Уверены, что хотите покинуть все объекты?"
        titleClassName="text-[28px] leading-8"
        description="Вы потеряете доступ ко всем объектам пользователей. Попросить доступ можно будет снова"
        descriptionClassName="text-base"
        confirmLabel="Покинуть все объекты"
        cancelLabel="Отмена"
        confirmVariant="danger"
        stacked
        pending={leaveAll.isPending}
        onConfirm={confirmLeaveAll}
      />

      {leftPopup === 'single' && (
        <ParticipantSuccessPopup
          title="Вы покинули объект"
          onClose={() => setLeftPopup(null)}
        />
      )}

      {leftPopup === 'all' && (
        <ParticipantSuccessPopup
          title="Вы покинули все объекты пользователей"
          onClose={() => setLeftPopup(null)}
        />
      )}
    </>
  );
}

/** Чип «Владелец объекта» (2010-133846): нейтральный, с замком — канон
 * owner=замок из списков объекта (#700). */
const ownerBadge: UserPropertyBadge = { tone: 'neutral', label: 'Владелец объекта', icon: 'lock' };

/** Ряд списка (2010-132145): фото или серый дом 44, титул, адрес, бейдж
 * роли; справа кебаб действий — ряд сам не кликабельный (навигации в
 * макете нет). */
function UserPropertyRowItem({
  row,
  onActions,
}: {
  readonly row: UserPropertyRow;
  readonly onActions: () => void;
}): JSX.Element {
  return (
    <div className="flex w-full items-center gap-3 py-3">
      <ObjectAvatarGlyph photoUrl={row.photoUrl} />
      <span className="flex min-w-0 flex-1 flex-col gap-1">
        <span className="truncate text-base font-medium leading-[18px] text-content">
          {row.title}
        </span>
        <span className="truncate text-sm leading-4 text-content-secondary">
          {row.address}
        </span>
        <span>
          <ParticipantRowBadge badge={row.badge} />
        </span>
      </span>
      <IconButton
        icon={<Kebab />}
        label={`Действия с объектом «${row.title}»`}
        onClick={onActions}
      />
    </div>
  );
}

/** Карточка объекта/владельца в шите действий (2010-133846): та же
 * анатомия «аватар — титул — подзаголовок — бейдж», без кебаба. */
function ObjectCard({
  photoUrl,
  glyph,
  title,
  subtitle,
  badge,
}: {
  readonly photoUrl?: string;
  readonly glyph?: JSX.Element;
  readonly title: string;
  readonly subtitle?: string;
  readonly badge: UserPropertyBadge;
}): JSX.Element {
  return (
    <div className="flex w-full items-center gap-3">
      {glyph !== undefined ? (
        <span
          aria-hidden
          className="flex h-11 w-11 shrink-0 items-center justify-center rounded-full bg-surface-muted"
        >
          {glyph}
        </span>
      ) : (
        <ObjectAvatarGlyph photoUrl={photoUrl} />
      )}
      <span className="flex min-w-0 flex-1 flex-col gap-1">
        <span className="truncate text-base font-medium leading-[18px] text-content">
          {title}
        </span>
        {subtitle !== undefined && (
          <span className="truncate text-sm leading-4 text-content-secondary">
            {subtitle}
          </span>
        )}
        <span>
          <ParticipantRowBadge badge={badge} />
        </span>
      </span>
    </div>
  );
}


/** Группы пикера сортировки «Название»: поле единственное, направление —
 * выбор применяется сразу (канон PickerMenu, «Ваши участники» #697). */
function sortPickerGroups(
  order: ParticipantsPropertySortOrder,
  onChange: (order: ParticipantsPropertySortOrder) => void,
): ReadonlyArray<PickerMenuGroup> {
  return [
    {
      options: [{ label: 'Название', selected: true, onSelect: () => undefined }],
    },
    {
      options: [
        { label: 'Возрастание', selected: order === 'asc', onSelect: () => onChange('asc') },
        { label: 'Убывание', selected: order === 'desc', onSelect: () => onChange('desc') },
      ],
    },
  ];
}
