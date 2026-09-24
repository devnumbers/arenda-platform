'use client';

import { useState, type JSX } from 'react';
import { useRouter } from 'next/navigation';
import { Add, Block, BoldUser, Info, Kebab, SmallArrowRight, TimeHistory } from '@/shared/assets/icons';
import { goBack } from '@/shared/lib/navigation';
import { ROUTES } from '@/shared/config/routes';
import {
  ParticipantRowBadge,
  participantLegBadge,
  participantStatusBadge,
  type Participant,
  type ParticipantPropertyLeg,
} from '@/entities/participants';
import { useParticipant, useRevokeParticipant } from '@/features/participants';
import { useProperties } from '@/features/properties';
import {
  ConfirmDialog,
  ErrorCard,
  IconButton,
  Menu,
  MenuContent,
  MenuItem,
  MenuTrigger,
  SubScreenShell,
} from '@/shared/ui/design';
import {
  ObjectAvatarGlyph,
  PARTICIPANT_ROW_BASE_CLASS,
  ParticipantNotFound,
} from './participant-fragments';
import { popParticipantPopup, stageParticipantPopup, useParticipantPopup } from '../lib/participant-popups';
import { ParticipantSuccessPopup } from './participant-success-popup';
import { ParticipantScreenSkeleton } from './participants-skeletons';

/**
 * Страница участника (карта #692, тикет #698; Figma 2008-81468): блок
 * «аватар — имя — почта — чип агрегат-статуса», список «Доступные объекты»
 * (Row Button с фото/домом, адресом из кэша /properties, бейджем роли ноги
 * и чевроном) и кебаб шапки: «Пригласить в объект» (#694, отдельный экран)
 * и «Отозвать доступ к объектам» (макет 2008-48318; ConfirmDialog 2008-82147
 * канона #629 — крупный заголовок, кнопки столбиком) → DELETE
 * /participants/{id} → goBack на список (источник), где рендерится попап
 * «Участник удален» (2008-83716; флаг — stageParticipantPopup).
 *
 * Пункт кебаба «Действия участника» — вход в журнал истории (#712, карта
 * «История действий»): та же лента, прибитая к человеку. Только у
 * зарегистрированных (userId известен = actor_id журнала): у
 * pending-приглашения действий не бывает.
 *
 * Deep-link-политика (#693): человек вне скоупа читающего или уже отозван
 * — приватный 404 → «Участник не найден» по центру, кебаб скрыт (§7),
 * «Назад» жив. Адреса рядов — обогащение из общего кэша GET /properties;
 * ноги сцопа всегда лежат в нём, при холодном кэше ряд просто без
 * подзаголовка.
 */
export function ParticipantScreen({
  participantId,
}: {
  readonly participantId: string;
}): JSX.Element {
  const router = useRouter();

  const participantQuery = useParticipant(participantId);
  const revoke = useRevokeParticipant();
  const propertiesQuery = useProperties();

  const [confirmRevokeOpen, setConfirmRevokeOpen] = useState(false);
  // Попап, принесённый возвратом с экрана приглашения/прав (#698):
  // 'deleted' здесь не бывает — его рендерит список.
  const stagedPopup = useParticipantPopup();
  const popup =
    stagedPopup === 'granted' || stagedPopup === 'revokedFromProperty'
      ? stagedPopup
      : null;

  const participant = participantQuery.data;
  const headerLoaded = participant !== undefined;
  // Иконки/меню шапки — только когда участник загружен (§7: пустых и
  // ошибочных состояний они не касаются).
  const kebabVisible = headerLoaded;
  // Пункт кебаба «Действия участника»: переход по uuid юзера — это и есть
  // actor_id журнала; у pending-строки userId нет.
  const actionsUserId = participant?.userId;
  const propertyById = new Map(
    (propertiesQuery.data ?? []).map((property) => [property.id, property]),
  );

  return (
    <>
      <SubScreenShell
        title="Участник"
        fallbackHref={ROUTES.participantsList}
        trailing={
          kebabVisible ? (
            <span className="flex items-center pr-3.5">
              <Menu>
                <MenuTrigger asChild>
                  <IconButton icon={<Kebab />} label="Еще — действия с участником" />
                </MenuTrigger>
                <MenuContent>
                  <MenuItem
                    icon={<Add className="h-6 w-6" />}
                    onSelect={() =>
                      router.push(ROUTES.participantInvite(participantId))
                    }
                  >
                    Пригласить в объект
                  </MenuItem>
                  {actionsUserId !== undefined && (
                    <MenuItem
                      icon={<TimeHistory className="h-6 w-6" />}
                      onSelect={() =>
                        router.push(ROUTES.historyParticipant(actionsUserId))
                      }
                    >
                      Действия участника
                    </MenuItem>
                  )}
                  <MenuItem
                    icon={<Block className="h-6 w-6 text-error" />}
                    className="text-error"
                    onSelect={() => setConfirmRevokeOpen(true)}
                  >
                    Отозвать доступ к объектам
                  </MenuItem>
                </MenuContent>
              </Menu>
            </span>
          ) : undefined
        }
      >
        {participantQuery.isPending ? (
          <ParticipantScreenSkeleton />
        ) : participantQuery.isError ? (
          participantQuery.error.status === 404 ? (
            <ParticipantNotFound />
          ) : (
            <ErrorCard
              title="Не удалось загрузить участника"
              onRetry={() => void participantQuery.refetch()}
              className="mt-6"
            />
          )
        ) : (
          <div className="flex flex-col gap-6 pb-2">
            <ParticipantHeader participant={participantQuery.data} />
            {participantQuery.data.aggregateStatus === 'limit_exceeded' && (
              <ParticipantLimitNotice />
            )}
            <section className="flex flex-col gap-2">
              <h2 className="text-xl font-semibold leading-6 text-content">
                Доступные объекты
              </h2>
              {participantQuery.data.properties.map((leg) => (
                <ParticipantPropertyRowButton
                  key={leg.propertyId}
                  leg={leg}
                  subtitle={propertyById.get(leg.propertyId)?.address}
                  photoUrl={propertyById.get(leg.propertyId)?.photos?.[0]?.url}
                  onSelect={() =>
                    router.push(
                      ROUTES.participantRights(participantId, leg.propertyId),
                    )
                  }
                />
              ))}
            </section>
          </div>
        )}
      </SubScreenShell>

      <ConfirmDialog
        open={confirmRevokeOpen}
        onOpenChange={setConfirmRevokeOpen}
        title="Отозвать у пользователя доступ ко всем вашим объектам?"
        titleClassName="text-[28px] leading-8"
        description="Пользователь потеряет доступ ко всем вашим объектам и будет удален из списка участников. Пригласить его можно будет снова"
        confirmLabel="Отозвать и удалить"
        cancelLabel="Отменить"
        confirmVariant="danger"
        stacked
        pending={revoke.isPending}
        onConfirm={() => {
          revoke.mutate(participantId, {
            onSuccess: () => {
              setConfirmRevokeOpen(false);
              // Источник страницы — список: возврат по канону истории,
              // попап «Участник удален» рендерит список (2008-83716).
              stageParticipantPopup('deleted');
              goBack(router, ROUTES.participantsList);
            },
          });
        }}
      />

      {/* Попапы, принесённые возвратом: «Пригласить в объект» (#698,
       * 2010-131859) и отзыв из объекта с экрана прав (#698, 2008-83135). */}
      {popup !== null && (
        <ParticipantSuccessPopup
          title={
            popup === 'granted'
              ? 'Доступ выдан'
              : 'У участника больше нет доступа к объекту'
          }
          onClose={() => popParticipantPopup()}
        />
      )}
    </>
  );
}

/** Блок «аватар — имя — почта — чип» (макет 2008-81468): аватар-плейсхолдер
 * 96 (BoldUser, запечённый серый), имя H1 28/32 SemiBold, почта 14/16 —
 * всё по центру; у pending-участника имени нет — почта уже титул. */
function ParticipantHeader({ participant }: { readonly participant: Participant }): JSX.Element {
  const title = participant.displayName ?? participant.email ?? '';
  const email =
    participant.displayName !== undefined ? participant.email : undefined;

  return (
    <div className="flex flex-col items-center gap-2">
      <span
        aria-hidden
        className="flex h-24 w-24 items-center justify-center rounded-pill bg-surface-muted"
      >
        <BoldUser className="h-[52px] w-[52px]" />
      </span>
      <h1 className="text-center text-[28px] font-semibold leading-8 text-content">
        {title}
      </h1>
      {email !== undefined && (
        <p className="text-center text-sm leading-4 text-content-secondary">{email}</p>
      )}
      <div className="pt-2">
        <ParticipantRowBadge badge={participantStatusBadge(participant)} />
      </div>
    </div>
  );
}

/** Жёлтая карточка-пояснение у suspended-участника (макет 2036-84861,
 * правка приёмки #756): почему участник пока не пользуется объектами и
 * что делать. Рисуется только при агрегате «Превышен лимит объектов» —
 * статус уже виден бейджем в шапке, ноги несут бейджи ролей. */
function ParticipantLimitNotice(): JSX.Element {
  return (
    <div
      className="flex items-start gap-3 rounded-[32px] bg-warning-bg p-6 pr-8"
      data-testid="participant-limit-notice"
    >
      <Info className="h-6 w-6 shrink-0 text-content" aria-hidden />
      <div className="flex min-w-0 flex-col gap-2">
        <p className="text-base font-medium leading-[18px] text-content">
          Пользователь пока не может пользоваться вашим объектом
        </p>
        <p className="text-sm leading-4 text-content">
          Попросите его освободить слот под ваш объект, чтобы он смог
          просматривать его и редактировать
        </p>
      </div>
    </div>
  );
}

/** Ряд «Доступных объектов» (макет 2008-81468, Row Button 936:39347):
 * фото или серый дом 44, титул, адрес, бейдж роли/состояния ноги, чеврон.
 * Тап — экран «Права участника» (#698, 2177-59620). */
function ParticipantPropertyRowButton({
  leg,
  subtitle,
  photoUrl,
  onSelect,
}: {
  readonly leg: ParticipantPropertyLeg;
  readonly subtitle: string | undefined;
  readonly photoUrl: string | undefined;
  readonly onSelect: () => void;
}): JSX.Element {
  return (
    <button
      type="button"
      onClick={onSelect}
      className={PARTICIPANT_ROW_BASE_CLASS}
    >
      <ObjectAvatarGlyph photoUrl={photoUrl} />
      <span className="flex min-w-0 flex-1 flex-col gap-1">
        <span className="truncate text-base font-medium leading-[18px] text-content">
          {leg.title}
        </span>
        {subtitle !== undefined && (
          <span className="truncate text-sm leading-4 text-content-secondary">
            {subtitle}
          </span>
        )}
        <span>
          <ParticipantRowBadge badge={participantLegBadge(leg)} />
        </span>
      </span>
      <SmallArrowRight className="h-6 w-6 shrink-0 text-content-tertiary" aria-hidden />
    </button>
  );
}
