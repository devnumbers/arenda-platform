'use client';

import { type JSX } from 'react';
import { useRouter } from 'next/navigation';
import NextLink from 'next/link';
import { cn } from '@/shared/lib/cn';
import { TimeHistory } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { ParticipantRowBadge, participantLegBadge } from '@/entities/participants';
import { usePropertyAccessMember } from '@/features/access';
import { useProperty } from '@/features/properties';
import { ErrorCard, SubScreenShell, UserAvatar } from '@/shared/ui/design';
import { LiveValue } from '@/shared/ui/live-value';
import { pendingMemberRightsCard, type ParticipantRightsCard } from '../lib/participant-rights-card';
import { resolvePropertyParticipantsError } from '../lib/property-participants-error';
import {
  EMAIL_ROLE_ICONS,
  ObjectAvatarGlyph,
  PARTICIPANT_ROW_BASE_CLASS,
  ParticipantNotFound,
  PropertyAccessNotFound,
  RIGHTS_ROW_ACTION_CLASS,
} from './participant-fragments';
import { ParticipantRightsMemberSkeleton } from './participants-skeletons';

/**
 * Экран «Права участника» в member-режиме (решение владельца 12.10.2026,
 * тикет #1362, канон макета 3296-75335): приглашённый участник объекта
 * открывает страницу прав любого ряда «Участников объекта», включая свою
 * строку и ряд владельца. Read-only по построению — данных для мутаций
 * нет и быть не должно: чтение одной строки
 * GET /properties/{id}/access/members/{memberId} (контракт #1357, тот же
 * гейт, что у списка) вместо owner-агрегата /participants/{id}, который
 * не-владельцу отвечает приватный 404. В отличие от владелецского экрана
 * (#698): без сегмента смены роли и без «Отозвать доступ к объекту» —
 * карточка объекта с бейджем роли и строка «Действия участника в
 * объекте» (переход на пару historyMemberProperty #841; у ряда владельца
 * бейдж «Владелец», журнал его действий тоже жив). Список ведёт сюда и
 * на архивном объекте: чтение строки живёт тем же гейтом, что и список
 * (участники, история и выход на архивном сохраняются) — ветка
 * isArchived владельческого тапа сюда не переносится. Карточка объекта —
 * ссылка на сам объект (амендмент владельца 13.10.2026, тот же на
 * владельческом экране): вид прежний, без шеврона.
 *
 * Pending-ряд («Аккаунт не создан») — карточка приглашения, строк
 * действий нет: uuid юзера (actor_id журнала) ещё не существует, роли
 * участник не меняет, отзывать нечем. Инфо-блок D7 («он сможет
 * пользоваться вашим объектом») здесь не воспроизводится — копия
 * адресована владельцу объекта, читатель member-режима — другой
 * участник.
 *
 * Маршрут адресуется memberId (адресующий id строки списка, клиентское
 * правило `row.id ?? row.user_id`) — deep-link и refresh участника
 * работают по построению; у pending-приглашения это invitation id,
 * поэтому почтой адресовать member-маршрут нельзя (в отличие от
 * владельческого /participants/{почта}). 404 разводится честно: объект
 * нечитаем целиком (чужой, отозван, suspended — канон #719) —
 * PropertyAccessNotFound; объект читается, строки нет (мусорный id, нога
 * отозвана в другой сессии) — «Участник не найден» (#693). Оба чтения
 * отвечают одним приватным 404, поэтому разводка ждёт разрешения
 * propertyQuery — иначе чужой объект успевал бы мигнуть «Участник не
 * найден» перед честным итогом.
 *
 * Бейдж роли оживает каноном C (#880), иконка роли у почты pending-
 * карточки — каноном B (плотные карточки), как на владельческом D7:
 * чужая смена роли гасит значение на перечитывание; своего edit здесь
 * нет — own-защёлке взяться неоткуда. Фолбэк «Назад» — «Участники
 * объекта» (источник входа).
 */
export function ParticipantRightsMemberScreen({
  propertyId,
  memberId,
}: {
  readonly propertyId: string;
  readonly memberId: string;
}): JSX.Element {
  const router = useRouter();

  const memberQuery = usePropertyAccessMember(propertyId, memberId);
  const propertyQuery = useProperty(propertyId);

  const member = memberQuery.data;
  const pendingCard =
    member !== undefined ? pendingMemberRightsCard(member) : undefined;
  const property = propertyQuery.data;
  // «Действия участника в объекте» (#841): вход в прибитую ленту — только
  // у зарегистрированных (uuid юзера = actor_id журнала); у pending
  // действий не бывает (канон кебаба страницы участника #712).
  const actionsUserId = member?.userId ?? undefined;

  let content: JSX.Element;
  if (memberQuery.isPending) {
    content = <ParticipantRightsMemberSkeleton />;
  } else if (memberQuery.isError) {
    content =
      resolvePropertyParticipantsError(memberQuery.error) !== 'not_found' ? (
        <ErrorCard
          title="Не удалось загрузить права участника"
          onRetry={() => void memberQuery.refetch()}
          loading={memberQuery.isFetching}
          className="mt-6"
        />
      ) : propertyQuery.isPending ? (
        // Строки нет, а объект ещё читается: итог (чужой объект или
        // мусорный id) знает только разрешившийся объект — скелетон вместо
        // мигающего «Участник не найден».
        <ParticipantRightsMemberSkeleton />
      ) : propertyQuery.isError &&
        resolvePropertyParticipantsError(propertyQuery.error) === 'not_found' ? (
        // Объект нечитаем целиком: чужой, свой доступ отозван/приостановлен
        // в открытой сессии (#719) — бэк скрывает его как 404, канон в доке
        // PropertyAccessNotFound.
        <PropertyAccessNotFound />
      ) : (
        // Объект читается, строки нет — мусорный memberId или нога отозвана
        // в другой сессии: честный приватный 404 (#693), как у агрегата
        // владелецского экрана.
        <ParticipantNotFound />
      );
  } else if (member !== undefined && pendingCard !== undefined) {
    content = (
      <ParticipantRightsMemberPending card={pendingCard} refreshing={memberQuery.isFetching} />
    );
  } else if (member !== undefined) {
    content = (
      <div className="flex flex-col gap-6">
        {/* Карточка объекта — ссылка на сам объект (амендмент владельца
          * 13.10.2026, общий с владельческим экраном): вид прежний, без
          * шеврона — курсор и ховер канонного кликабельного ряда. */}
        <NextLink
          href={ROUTES.property(propertyId)}
          className={PARTICIPANT_ROW_BASE_CLASS}
          data-testid="participant-rights-object-card"
        >
          <ObjectAvatarGlyph
            photoUrl={property?.photoUrl ?? undefined}
            type={property?.type}
          />
          <span className="flex min-w-0 flex-1 flex-col gap-1">
            <span className="truncate text-base font-medium leading-[18px] text-content">
              {property?.name ?? ''}
            </span>
            {property?.address !== undefined && (
              <span className="truncate text-sm leading-4 text-content-secondary">
                {property.address}
              </span>
            )}
            <span>
              {/* Бейдж роли оживает каноном C (#880): чужая смена роли
                * (владелец в другой сессии) гасит бейдж на перечитывание и
                * проявляет новое значение кроссфейдом. */}
              <LiveValue valueKey={`${member.status}-${member.role}`} refreshing={memberQuery.isFetching}>
                <ParticipantRowBadge badge={participantLegBadge(member)} />
              </LiveValue>
            </span>
          </span>
        </NextLink>

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
        </div>
      </div>
    );
  } else {
    // Строка пришла пустой — не бывает (успех всегда несёт тело); ветка
    // полноты свитча.
    content = <ParticipantNotFound />;
  }

  return (
    <SubScreenShell
      title="Права участника"
      fallbackHref={ROUTES.propertyParticipants(propertyId)}
    >
      {content}
    </SubScreenShell>
  );
}

/**
 * Карточка pending-приглашения в member-режиме (#1362): та же анатомия
 * D7-карточки владельческого экрана (3298-75576) — аватар, анонимный
 * титул «Пользователь», почта с иконкой роли, бейдж «Аккаунт не создан» —
 * но ряд статичный: чеврона на страницу участника нет (агрегат
 * owner-only, member получил бы приватный 404), сегмента и отзыва нет
 * (мутации не для member-режима). Иконка роли оживает каноном B
 * (#880), как на владельческой D7-карточке.
 */
function ParticipantRightsMemberPending({
  card,
  refreshing,
}: {
  readonly card: ParticipantRightsCard;
  readonly refreshing: boolean;
}): JSX.Element {
  const EmailIcon = EMAIL_ROLE_ICONS[card.emailIcon];

  return (
    <section
      className={cn(PARTICIPANT_ROW_BASE_CLASS, 'cursor-default')}
      data-testid="participant-rights-card"
    >
      <UserAvatar photoUrl={card.photoUrl ?? null} />
      <span className="flex min-w-0 flex-1 flex-col gap-1">
        <span className="truncate text-base font-medium leading-[18px] text-content">
          {card.title}
        </span>
        <span className="flex min-w-0 items-center gap-1 text-sm text-content-secondary">
          <LiveValue valueKey={card.emailIcon} mode="flash" refreshing={refreshing}>
            <EmailIcon className="h-4 w-4 shrink-0" aria-hidden />
          </LiveValue>
          <span className="truncate">{card.email}</span>
        </span>
        <span>
          <ParticipantRowBadge badge={card.badge} />
        </span>
      </span>
    </section>
  );
}
