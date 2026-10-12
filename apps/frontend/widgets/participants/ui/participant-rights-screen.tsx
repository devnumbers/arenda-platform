'use client';

import { useEffect, useRef, useState, type JSX } from 'react';
import { useRouter } from 'next/navigation';
import { cn } from '@/shared/lib/cn';
import { Block, EyeSmall, FullAccessSmall, Info, SmallArrowRight, TimeHistory } from '@/shared/assets/icons';
import { goBack } from '@/shared/lib/navigation';
import { ROUTES } from '@/shared/config/routes';
import {
  ParticipantRowBadge,
  participantLegBadge,
  type ParticipantAccessRole,
} from '@/entities/participants';
import type { PropertyAccessMember } from '@/entities/access';
import { useParticipant } from '@/features/participants';
import { useProperties } from '@/features/properties';
import {
  useCancelPropertyAccessInvitation,
  useDeletePropertyAccessMember,
  usePropertyAccessMembers,
  useUpdatePropertyAccessInvitation,
  useUpdatePropertyAccessMember,
} from '@/features/access';
import {
  ConfirmDialog,
  ErrorCard,
  SubScreenShell,
  UserAvatar,
} from '@/shared/ui/design';
import { LiveValue } from '@/shared/ui/live-value';
import { resolveParticipantMemberRow } from '../lib/participant-member-lookup';
import { resolvePropertyParticipantsError } from '../lib/property-participants-error';
import { pendingParticipantRightsCard, type ParticipantRightsCard } from '../lib/participant-rights-card';
import {
  ObjectAvatarGlyph,
  PARTICIPANT_ROW_BASE_CLASS,
  ParticipantNotFound,
  PropertyAccessNotFound,
} from './participant-fragments';
import { stageParticipantPopup } from '../lib/participant-popups';
import { ParticipantSuccessPopup } from './participant-success-popup';
import { ParticipantRoleSegmented } from './participant-role-segmented';
import { ParticipantRightsSkeleton } from './participants-skeletons';

/** Row Button нижних строк макета 2177-59620: ведущий слот-круг 44 под
 * иконку 24, подпись 16/18 Medium; высота строки 52 (44 + py 4). Общая
 * для «Действий участника в объекте» и красного отзыва — иконки обеих
 * строк стоят на одной оси. */
const RIGHTS_ROW_ACTION_CLASS = cn(
  'flex w-full cursor-pointer items-center rounded-button py-1 text-left text-base font-medium outline-none',
  'transition-opacity hover:opacity-80 focus-visible:ring-4 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-surface active:opacity-80',
);

/** Окно «своего» изменения бейджа роли после локальной мутации (мс):
 * инвалидация onSettled приносит перечитывание с уже новым значением —
 * своя правка не анимируется (канон #880: вспышка и dim — сигнал «кто-то
 * другой поменял»); окно с запасом покрывает перечитывание. */
const OWN_CHANGE_WINDOW_MS = 3000;

/**
 * Экран «Права участника» (карта #692, тикет #698; Figma 2177-59620):
 * шапка «назад + Права участника», строка объекта с бейджем роли,
 * сегмент-переключатель «Просмотр | Редактирование» (активная роль — белая
 * с тенью), строка «Действия участника в объекте» — вход в ленту истории,
 * прибитую к паре человек+объект (#841, тот же макет; только у
 * зарегистрированных — у pending действий не бывает, канон кебаба
 * страницы участника #712) — и красная строка «Отозвать доступ к объекту»;
 * обе строки — Row Button макета с ведущим слотом-кругом 44 под иконку 24.
 *
 * D7 — приглашённый без аккаунта (#1314, #1264; макет 3298-75576): вместо
 * карточки объекта — карточка участника (анонимный титул «Пользователь»,
 * почта с иконкой роли, бейдж «Аккаунт не создан», чеврон — на страницу
 * участника), голубой инфо-блок «Пользователь еще не создал аккаунт /
 * После создания аккаунта он сможет пользоваться вашим объектом», тот же
 * сегмент роли (мутации — только собственник: агрегат owner-скоупа D5
 * #1304 — не-собственнику экран отвечает приватным 404, сегмента и отзыва
 * он не видит) и красная строка «Отозвать доступ» (отмена приглашения).
 * Строки «Действия участника» у pending нет: журнал адресуется uuid юзера
 * (actor_id), которого не существует — канон #712; на макете строка —
 * общий шаблон, для pending она не наполнена.
 *
 * Данные: агрегат GET /participants/{id} и строки
 * GET /properties/{id}/access/members; участник связывается со строкой
 * по user_id (pending-приглашение — по email). Роль уходит в PATCH
 * members (pending — invitations), отзыв — DELETE members (pending —
 * DELETE invitations); после мутации агрегаты инвалидируются (чипы ролей
 * на странице участника и в списке перечитаются). Успех роли — попап
 * «Права изменены» (2177-59799); успех отзыва — переход на страницу
 * участника с попапом «У участника больше нет доступа к объекту»
 * (2008-83135, staged-флаг participant-popups — канон one-shot #771).
 * Если это был последний объект,
 * страница участника честно покажет «Участник не найден» — контракт
 * приватного 404 (#693). Строки нет уже на этом экране (нога отозвана
 * в другой сессии) — тот же честный текст без «Повторить».
 */
export function ParticipantRightsScreen({
  participantId,
  propertyId,
}: {
  readonly participantId: string;
  readonly propertyId: string;
}): JSX.Element {
  const router = useRouter();

  const participantQuery = useParticipant(participantId);
  const membersQuery = usePropertyAccessMembers(propertyId);
  const propertiesQuery = useProperties();

  const updateMember = useUpdatePropertyAccessMember(propertyId);
  const updateInvitation = useUpdatePropertyAccessInvitation(propertyId);
  const deleteMember = useDeletePropertyAccessMember(propertyId);
  const cancelInvitation = useCancelPropertyAccessInvitation(propertyId);

  const [showRoleChanged, setShowRoleChanged] = useState(false);
  const [confirmRevokeOpen, setConfirmRevokeOpen] = useState(false);
  // Защёлка «своего» изменения (канон #880): своя смена роли — мгновенно,
  // без dim/вспышки на бейдже; чужая правка (второй manage-читатель)
  // анимируется. Булев latch с таймером — рендер остаётся чистым.
  const [ownChangeLatch, setOwnChangeLatch] = useState(false);
  const ownLatchTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  useEffect(() => {
    return () => {
      if (ownLatchTimerRef.current !== null) {
        clearTimeout(ownLatchTimerRef.current);
      }
    };
  }, []);

  const participant = participantQuery.data;
  const member: PropertyAccessMember | undefined = participant
    ? resolveParticipantMemberRow(membersQuery.data ?? [], participant)
    : undefined;
  // D7-кейс — приглашённый без аккаунта (pending-строка): другой макет
  // (3298-75576) и другой набор строк; мутации те же (invitations).
  const pendingCard =
    participant !== undefined && member !== undefined
      ? pendingParticipantRightsCard(participant, member)
      : undefined;
  const property = propertiesQuery.data?.find((item) => item.id === propertyId);
  const leg = participant?.properties.find((item) => item.propertyId === propertyId);
  // «Действия участника в объекте» (#841): вход в прибитую ленту — только
  // у зарегистрированных (uuid юзера = actor_id журнала); у pending
  // действий не бывает (канон кебаба страницы участника #712).
  const actionsUserId = participant?.userId;

  const mutationPending =
    updateMember.isPending ||
    updateInvitation.isPending ||
    deleteMember.isPending ||
    cancelInvitation.isPending;

  const changeRole = (role: ParticipantAccessRole): void => {
    if (member === undefined || member.id === null || member.role === role) {
      return;
    }
    // Инвалидацию обеих семей делает access-хук (onSettled) — здесь только
    // попап и защёлка «своего» изменения.
    const onSuccess = (): void => {
      setOwnChangeLatch(true);
      if (ownLatchTimerRef.current !== null) {
        clearTimeout(ownLatchTimerRef.current);
      }
      ownLatchTimerRef.current = setTimeout(() => setOwnChangeLatch(false), OWN_CHANGE_WINDOW_MS);
      setShowRoleChanged(true);
    };
    if (member.status === 'pending') {
      updateInvitation.mutate({ invitationId: member.id, role }, { onSuccess });
    } else {
      updateMember.mutate({ memberId: member.id, role }, { onSuccess });
    }
  };

  const revoke = (): void => {
    if (member === undefined || member.id === null) {
      return;
    }
    // Инвалидацию обеих семей делает access-хук (onSettled) — здесь только
    // навигация и попап.
    const onDone = (): void => {
      // Источник — страница участника: возврат по канону истории, попап
      // «У участника больше нет доступа к объекту» рендерит он (2008-83135).
      stageParticipantPopup('revokedFromProperty');
      goBack(router, ROUTES.participant(participantId));
    };
    if (member.status === 'pending') {
      cancelInvitation.mutate(member.id, { onSuccess: onDone });
    } else {
      deleteMember.mutate(member.id, { onSuccess: onDone });
    }
  };

  let content: JSX.Element;
  if (participantQuery.isPending || membersQuery.isPending) {
    content = <ParticipantRightsSkeleton />;
  } else if (participantQuery.isError) {
    content =
      participantQuery.error.status === 404 ? (
        <ParticipantNotFound />
      ) : (
        <ErrorCard
          title="Не удалось загрузить участника"
          onRetry={() => void participantQuery.refetch()}
          className="mt-6"
        />
      );
  } else if (membersQuery.isError) {
    content =
      resolvePropertyParticipantsError(membersQuery.error) === 'not_found' ? (
        // Свой доступ отозван/приостановлен в открытой сессии (#719):
        // бэк скрывает нечитаемый объект как 404 — канон в доке
        // PropertyAccessNotFound.
        <PropertyAccessNotFound />
      ) : (
        <ErrorCard
          title="Не удалось загрузить права участника"
          onRetry={() => void membersQuery.refetch()}
          className="mt-6"
        />
      );
  } else if (member === undefined) {
    // Список успешен, а строки нет — нога уже отозвана (контракт
    // participant-member-lookup: «строки нет — доступ уже снят»); честный
    // «Участник не найден», как у агрегата выше — приватный 404 (#693).
    content = <ParticipantNotFound />;
  } else if (pendingCard !== undefined) {
    content = (
      <ParticipantRightsPending
        card={pendingCard}
        role={member.role === 'full_access' ? 'full_access' : 'viewer'}
        disabled={mutationPending}
        refreshing={participantQuery.isFetching}
        own={mutationPending || ownChangeLatch}
        onChangeRole={changeRole}
        onRevoke={() => setConfirmRevokeOpen(true)}
        onOpenParticipant={() => router.push(ROUTES.participant(participantId))}
      />
    );
  } else {
    content = (
      <div className="flex flex-col gap-6">
        <section className={cn(PARTICIPANT_ROW_BASE_CLASS, 'cursor-default')}>
          <ObjectAvatarGlyph photoUrl={leg?.photoUrl ?? property?.photoUrl ?? undefined} type={leg?.type ?? property?.type} />
          <span className="flex min-w-0 flex-1 flex-col gap-1">
            <span className="truncate text-base font-medium leading-[18px] text-content">
              {leg?.title ?? property?.name ?? ''}
            </span>
            {property?.address !== undefined && (
              <span className="truncate text-sm leading-4 text-content-secondary">
                {property.address}
              </span>
            )}
            {leg !== undefined && (
              <span>
                {/* Бейдж роли оживает каноном C (#880): чужая смена роли
                  * (кадр access инвалидирует агрегат — #719) гасит бейдж на
                  * перечитывание и проявляет новое значение кроссфейдом;
                  * своя правка (защёлка выше) — мгновенно. */}
                <LiveValue
                  valueKey={`${leg.status}-${leg.role}`}
                  refreshing={participantQuery.isFetching}
                  own={mutationPending || ownChangeLatch}
                >
                  <ParticipantRowBadge badge={participantLegBadge(leg)} />
                </LiveValue>
              </span>
            )}
          </span>
        </section>

        <ParticipantRoleSegmented
          value={member.role === 'full_access' ? 'full_access' : 'viewer'}
          disabled={mutationPending}
          onChange={changeRole}
          ariaLabel="Роль участника на объекте"
        />

        <div className="flex flex-col">
          {actionsUserId !== undefined && (
            <button
              type="button"
              onClick={() => router.push(ROUTES.historyMemberProperty(actionsUserId, propertyId))}
              className={RIGHTS_ROW_ACTION_CLASS}
              data-testid="participant-property-actions"
            >
              <span className="flex h-11 w-11 shrink-0 items-center justify-center rounded-pill">
                <TimeHistory className="h-6 w-6" aria-hidden />
              </span>
              <span className="pl-1 leading-[18px]">Действия участника в объекте</span>
            </button>
          )}
          <button
            type="button"
            onClick={() => setConfirmRevokeOpen(true)}
            className={cn(RIGHTS_ROW_ACTION_CLASS, 'text-error')}
            data-testid="participant-property-revoke"
          >
            <span className="flex h-11 w-11 shrink-0 items-center justify-center rounded-pill">
              <Block className="h-6 w-6" aria-hidden />
            </span>
            <span className="pl-1 leading-[18px]">Отозвать доступ к объекту</span>
          </button>
        </div>
      </div>
    );
  }

  return (
    <>
      <SubScreenShell
        title="Права участника"
        fallbackHref={ROUTES.participant(participantId)}
      >
        {content}
      </SubScreenShell>

      <ConfirmDialog
        open={confirmRevokeOpen}
        onOpenChange={setConfirmRevokeOpen}
        title="Уверены, что хотите отозвать доступ?"
        description="Пользователь потеряет доступ к объекту, пригласить его можно будет снова"
        confirmLabel="Отозвать"
        cancelLabel="Отмена"
        confirmVariant="danger"
        pending={
          deleteMember.isPending || cancelInvitation.isPending
        }
        onConfirm={revoke}
      />

      {showRoleChanged && (
        <ParticipantSuccessPopup
          title="Права изменены"
          onClose={() => setShowRoleChanged(false)}
        />
      )}
    </>
  );
}

/** Иконка роли у почты D7-карточки — та же дискриминация, что в рядах
 * «Участников объекта» (D6, 3298-75434): полный доступ — открытый замок,
 * просмотр — глаз. */
const PENDING_EMAIL_ICONS = {
  full: FullAccessSmall,
  eye: EyeSmall,
} as const;

/**
 * Контент «Права участника» без аккаунта (D7 #1314, макет 3298-75576):
 * карточка участника (Row Button — аватар, анонимный титул «Пользователь»,
 * почта с иконкой роли, бейдж «Аккаунт не создан», чеврон — на страницу
 * участника), инфо-блок про ещё не созданный аккаунт (голубая карточка
 * radius 32, канон жёлтой карточки лимита 2036-84861 в info-тоне),
 * сегмент роли и красная строка «Отозвать доступ» (отмена приглашения).
 * Иконка роли оживает каноном B (#880, как ряды D6): своя правка — без
 * анимации (own-защёлка экрана), чужая — вспышка.
 */
function ParticipantRightsPending({
  card,
  role,
  disabled,
  refreshing,
  own,
  onChangeRole,
  onRevoke,
  onOpenParticipant,
}: {
  readonly card: ParticipantRightsCard;
  readonly role: ParticipantAccessRole;
  readonly disabled: boolean;
  readonly refreshing: boolean;
  readonly own: boolean;
  readonly onChangeRole: (role: ParticipantAccessRole) => void;
  readonly onRevoke: () => void;
  readonly onOpenParticipant: () => void;
}): JSX.Element {
  const EmailIcon = PENDING_EMAIL_ICONS[card.emailIcon];

  return (
    <div className="flex flex-col gap-6">
      <button
        type="button"
        onClick={onOpenParticipant}
        className={PARTICIPANT_ROW_BASE_CLASS}
        data-testid="participant-rights-card"
      >
        <UserAvatar photoUrl={card.photoUrl ?? null} />
        <span className="flex min-w-0 flex-1 flex-col gap-1">
          <span className="truncate text-base font-medium leading-[18px] text-content">
            {card.title}
          </span>
          <span className="flex min-w-0 items-center gap-1 text-sm text-content-secondary">
            <LiveValue valueKey={card.emailIcon} mode="flash" refreshing={refreshing} own={own}>
              <EmailIcon className="h-4 w-4 shrink-0" aria-hidden />
            </LiveValue>
            <span className="truncate">{card.email}</span>
          </span>
          <span>
            <ParticipantRowBadge badge={card.badge} />
          </span>
        </span>
        <SmallArrowRight className="h-6 w-6 shrink-0 text-content-tertiary" aria-hidden />
      </button>

      <div
        className="flex items-start gap-3 rounded-[32px] bg-surface-info p-6 pr-8"
        data-testid="participant-pending-notice"
      >
        <Info className="h-6 w-6 shrink-0 text-primary" aria-hidden />
        <div className="flex min-w-0 flex-col gap-2">
          <p className="text-base font-medium leading-[18px] text-content">
            Пользователь еще не создал аккаунт
          </p>
          <p className="text-sm leading-4 text-content">
            После создания аккаунта он сможет пользоваться вашим объектом
          </p>
        </div>
      </div>

      <ParticipantRoleSegmented
        value={role}
        disabled={disabled}
        onChange={onChangeRole}
        ariaLabel="Роль участника на объекте"
      />

      <button
        type="button"
        onClick={onRevoke}
        className={cn(RIGHTS_ROW_ACTION_CLASS, 'text-error')}
        data-testid="participant-rights-revoke"
      >
        <span className="flex h-11 w-11 shrink-0 items-center justify-center rounded-pill">
          <Block className="h-6 w-6" aria-hidden />
        </span>
        <span className="pl-1 leading-[18px]">Отозвать доступ</span>
      </button>
    </div>
  );
}
