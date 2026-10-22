'use client';

import { useState, type JSX } from 'react';
import { useRouter } from 'next/navigation';
import { Add, Block, Info, Kebab, SmallArrowRight, TimeHistory } from '@/shared/assets/icons';
import { goBack, pushPage } from '@/shared/lib/navigation';
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
import type { PropertyType } from '@/entities/property';
import {
  ConfirmDialog,
  ErrorCard,
  IconButton,
  Menu,
  MenuContent,
  MenuItem,
  MenuTrigger,
  SubScreenShell,
  UserAvatar,
} from '@/shared/ui/design';
import {
  ObjectAvatarGlyph,
  PARTICIPANT_ROW_BASE_CLASS,
  ParticipantNotFound,
} from './participant-fragments';
import { notify } from '@/shared/lib/notifications';
import { ParticipantScreenSkeleton } from './participants-skeletons';

/**
 * Страница участника (карта #692, тикет #698; Figma 2008-81468): блок
 * «аватар — имя — почта — чип агрегат-статуса», список «Доступные объекты»
 * (Row Button с фото/домом, адресом из кэша /properties, бейджем роли ноги
 * и чевроном) и кебаб шапки: «Пригласить в объект» (#694, отдельный экран),
 * «История действий» (карта #1582, решение владельца 22.10.2026 в
 * переработке: «страница истории действий участника (только одного), где
 * будут все объекты, к которым я добавил ему доступ; чужих там объектов не
 * будет» — отдельная лента /participants/{id}/history, скоуп «человек × его
 * текущие ноги» выводит сервер) и «Отозвать доступ к объектам» (макет
 * 2008-48318; ConfirmDialog 2008-82147 канона #629 — крупный заголовок,
 * кнопки столбиком) → DELETE /participants/{id} → goBack на список
 * (источник), тост «Участник удален» показывается в момент успеха
 * (2008-83716; тост вместо успех-попапа, решение владельца 19.10.2026).
 *
 * Пункт кебаба «Действия участника» снимался вместе с глобальной страницей
 * (решение владельца 12.10.2026, #1361); возврат 22.10.2026 — НЕ его
 * воскрешение: путь /history/participants/[id] остался за перманентным
 * 308, пункт ведёт на под-ресурс страницы участника со скоупом ног
 * (registered и pending: у pending лента честно пуста — ручных действий не
 * бывает). Пункт живёт только при ногах (канон §7: входа в ленту без
 * скоупа не бывает).
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

  const participant = participantQuery.data;
  const headerLoaded = participant !== undefined;
  // Иконки/меню шапки — только когда участник загружен (§7: пустых и
  // ошибочных состояний они не касаются).
  const kebabVisible = headerLoaded;
  const propertyById = new Map(
    (propertiesQuery.data ?? []).map((property) => [property.id, property]),
  );

  return (
    <>
      <SubScreenShell
        title="Участник"
        fallbackHref={ROUTES.participants}
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
                      pushPage(router, ROUTES.participantInvite(participantId))
                    }
                  >
                    Пригласить в объект
                  </MenuItem>
                  {/* Вход «Истории действий» — только при ногах (канон §7:
                    * входа в ленту без скоупа не бывает); ноги есть у
                    * всякого загруженного агрегата (контракт: хотя бы
                    * одна). participant ненулевой по сужению kebabVisible
                    * (загруженная шапка) — ветка внутри него. */}
                  {participant.properties.length > 0 && (
                    <MenuItem
                      icon={<TimeHistory className="h-6 w-6" />}
                      onSelect={() =>
                        pushPage(router, ROUTES.participantHistory(participantId))
                      }
                    >
                      История действий
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
              loading={participantQuery.isFetching}
              className="mt-6"
            />
          )
        ) : (
          <div className="flex flex-col gap-12 pb-2">
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
                  photoUrl={leg.photoUrl ?? undefined}
                  type={leg.type}
                  onSelect={() =>
                    pushPage(router,
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
              // Тост в момент успеха — виден и на списке после возврата
              // по канону истории (goBack ниже).
              notify.scenarios.access.revoked();
              goBack(router, ROUTES.participants);
            },
          });
        }}
      />
    </>
  );
}

/** Блок «аватар — имя — почта — бейдж» (макеты 2008-81468, 2036-84861):
 * аватар 96 — фото профиля или заглушка BoldUser (UserAvatar hero), имя
 * H1 28/32 SemiBold, почта 14/16 — зазоры: аватар → блок 24, имя → почта
 * 8, блок → бейдж 24 + 6 внутреннего паддинга инстанса бейджа (pt-1.5);
 * всё по центру. Маржин h1 глобально снесён (#1402) — зазор тут явный
 * gap-2, легаси-плюс 18.76px больше не вспухает. У pending-участника
 * имени нет — почта уже титул. Бейдж — из двух статусных (D3 #1313):
 * «Аккаунт не создан» у pending, «Превышен лимит объектов» у suspended;
 * остальные без бейджа. */
function ParticipantHeader({ participant }: { readonly participant: Participant }): JSX.Element {
  const title = participant.displayName ?? participant.email ?? '';
  const email =
    participant.displayName !== undefined ? participant.email : undefined;
  const badge = participantStatusBadge(participant);

  return (
    <div className="flex flex-col items-center gap-6">
      <UserAvatar size="hero" photoUrl={participant.photoUrl} />
      <div className="flex flex-col items-center gap-2">
        <h1 className="text-center text-[28px] font-semibold leading-8 text-content">
          {title}
        </h1>
        {email !== undefined && (
          <p className="text-center text-sm leading-4 text-content-secondary">{email}</p>
        )}
      </div>
      {badge !== null && (
        <div className="pt-1.5">
          <ParticipantRowBadge badge={badge} />
        </div>
      )}
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
 * фото или глиф типа 44, титул, адрес, бейдж роли/состояния ноги, чеврон.
 * Тап — экран «Права участника» (#698, 2177-59620). */
function ParticipantPropertyRowButton({
  leg,
  subtitle,
  photoUrl,
  type,
  onSelect,
}: {
  readonly leg: ParticipantPropertyLeg;
  readonly subtitle: string | undefined;
  readonly photoUrl: string | undefined;
  readonly type: PropertyType | undefined;
  readonly onSelect: () => void;
}): JSX.Element {
  return (
    <button
      type="button"
      onClick={onSelect}
      className={PARTICIPANT_ROW_BASE_CLASS}
    >
      <ObjectAvatarGlyph photoUrl={photoUrl} type={type} />
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
