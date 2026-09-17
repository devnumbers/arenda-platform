'use client';

import { useState, type JSX } from 'react';
import { useRouter } from 'next/navigation';
import { useQueryClient } from '@tanstack/react-query';
import { cn } from '@/shared/lib/cn';
import { Block } from '@/shared/assets/icons';
import { goBack } from '@/shared/lib/navigation';
import { ROUTES } from '@/shared/config/routes';
import {
  ParticipantLegBadge,
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
import { participantsKeys } from '@/shared/api/query-keys';
import {
  ConfirmDialog,
  ErrorCard,
  SubScreenShell,
} from '@/shared/ui/design';
import { resolveParticipantMemberRow } from '../lib/participant-member-lookup';
import {
  ObjectAvatarGlyph,
  PARTICIPANT_ROW_BASE_CLASS,
  ParticipantNotFound,
} from './participant-fragments';
import { stageParticipantPopup } from '../lib/participant-popups';
import { ParticipantSuccessPopup } from './participant-success-popup';
import { ParticipantRoleSegmented } from './participant-role-segmented';
import { ParticipantRightsSkeleton } from './participants-skeletons';

/**
 * Экран «Права участника» (карта #692, тикет #698; Figma 2177-59620):
 * шапка «назад + Права участника», строка объекта с бейджем роли,
 * сегмент-переключатель «Просмотр | Редактирование» (активная роль — белая
 * с тенью) и красная строка «Отозвать доступ к объекту». Пункт макета
 * «Действия участника в объекте» — вход в журнал (#712, карта «История
 * действий»), вне скоупа #698.
 *
 * Данные: агрегат GET /participants/{id} и строки
 * GET /properties/{id}/access/members; участник связывается со строкой
 * по user_id (pending-приглашение — по email). Роль уходит в PATCH
 * members (pending — invitations), отзыв — DELETE members (pending —
 * DELETE invitations); после мутации агрегаты инвалидируются (чипы ролей
 * на странице участника и в списке перечитаются). Успех роли — попап
 * «Права изменены» (2177-59799); успех отзыва — переход на страницу
 * участника с попапом «У участника больше нет доступа к объекту»
 * (2008-83135, через search-param). Если это был последний объект,
 * страница участника честно покажет «Участник не найден» — контракт
 * приватного 404 (#693).
 */
export function ParticipantRightsScreen({
  participantId,
  propertyId,
}: {
  readonly participantId: string;
  readonly propertyId: string;
}): JSX.Element {
  const router = useRouter();
  const queryClient = useQueryClient();

  const participantQuery = useParticipant(participantId);
  const membersQuery = usePropertyAccessMembers(propertyId);
  const propertiesQuery = useProperties();

  const updateMember = useUpdatePropertyAccessMember(propertyId);
  const updateInvitation = useUpdatePropertyAccessInvitation(propertyId);
  const deleteMember = useDeletePropertyAccessMember(propertyId);
  const cancelInvitation = useCancelPropertyAccessInvitation(propertyId);

  const [showRoleChanged, setShowRoleChanged] = useState(false);
  const [confirmRevokeOpen, setConfirmRevokeOpen] = useState(false);

  const participant = participantQuery.data;
  const member: PropertyAccessMember | undefined = participant
    ? resolveParticipantMemberRow(membersQuery.data ?? [], participant)
    : undefined;
  const property = propertiesQuery.data?.find((item) => item.id === propertyId);
  const leg = participant?.properties.find((item) => item.propertyId === propertyId);

  const mutationPending =
    updateMember.isPending ||
    updateInvitation.isPending ||
    deleteMember.isPending ||
    cancelInvitation.isPending;

  const changeRole = (role: ParticipantAccessRole): void => {
    if (member === undefined || member.id === null || member.role === role) {
      return;
    }
    const onSuccess = (): void => {
      void queryClient.invalidateQueries({ queryKey: participantsKeys.all });
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
    const onDone = (): void => {
      void queryClient.invalidateQueries({ queryKey: participantsKeys.all });
      // Источник — страница участника: возврат по канону истории, попап
      // «У участника больше нет доступа к объекту» рендерит она (2008-83135).
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
  } else if (member === undefined) {
    content = (
      <ErrorCard
        title="Не удалось загрузить права участника"
        onRetry={() => void membersQuery.refetch()}
        className="mt-6"
      />
    );
  } else {
    content = (
      <div className="flex flex-col gap-6">
        <section className={cn(PARTICIPANT_ROW_BASE_CLASS, 'cursor-default')}>
          <ObjectAvatarGlyph photoUrl={property?.photos?.[0]?.url} />
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
                <ParticipantLegBadge badge={participantLegBadge(leg)} />
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

        <button
          type="button"
          onClick={() => setConfirmRevokeOpen(true)}
          className={cn(
            'flex w-full items-center gap-2 rounded-button py-3 text-left text-base font-medium text-error outline-none',
            'transition-opacity hover:opacity-80 focus-visible:ring-4 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-surface active:opacity-80',
          )}
        >
          <Block className="h-6 w-6 shrink-0" aria-hidden />
          Отозвать доступ к объекту
        </button>
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
