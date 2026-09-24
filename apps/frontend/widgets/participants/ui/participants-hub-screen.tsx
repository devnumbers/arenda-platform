'use client';

import type { ComponentType, JSX } from 'react';
import Image from 'next/image';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { Objects, SmallArrowRight, Team, TeamAdd } from '@/shared/assets/icons';
import { useParticipantsSummary } from '@/features/participants';
import { ROUTES } from '@/shared/config/routes';
import {
  Button,
  ErrorCard,
  HubCollapseAnchor,
  HubTitle,
  IconButton,
  PageContent,
  StickyBottomBar,
  TopNav,
} from '@/shared/ui/design';
import {
  participantsCountLabel,
  sharedPropertiesCountLabel,
} from '../lib/participants-counters';
import { popParticipantPopup, useParticipantPopup } from '../lib/participant-popups';
import { ParticipantsHubSkeleton } from './participants-hub-skeletons';
import { ParticipantSuccessPopup } from './participant-success-popup';

/**
 * Хаб «Совместный доступ» (карта #692, тикет #696; Figma 2008-47013 —
 * со счётчиками, 1967-86441 — пустой): карточки «Ваши участники» и
 * «Объекты пользователей» с счётчиками summary-эндпоинта (#693), ноль —
 * «Нет участников»/«Нет объектов»; CTA «Пригласить участника» —
 * постоянная нижняя панель. Входы — пункт навбара «Участники» и строка
 * в профиле; шапка — канон хаба с «крыльями» и на мобайле (#556),
 * иконка-приглашение — в строке заголовка и в компакт-баре (как «+»
 * «Задач» #523). 3D-иллюстрации карточек — общие с «Возможностями»
 * «О тарифе» (тот же артефакт Figma «Новые экраны сервиса»).
 *
 * Строка-вход «История действий» (#709, решение владельца 22.09) снята
 * по макету 1967-86441 (решение владельца 24.09, #843) — вход в ленту
 * переехал в кебабы «Ваших участников» и «Объектов пользователей».
 */
export function ParticipantsHubScreen(): JSX.Element {
  const router = useRouter();
  const summaryQuery = useParticipantsSummary();

  // «Участник приглашен» (2010-134458, #699): приглашение открывают и с
  // хаба — возврат по канону истории может привести сюда, попап не теряется.
  const invitedPopup = useParticipantPopup() === 'invited';

  const inviteButton = (
    <IconButton
      icon={<TeamAdd />}
      label="Пригласить участника"
      onClick={() => router.push(ROUTES.participantsInvite)}
    />
  );

  return (
    <>
      <TopNav
        mobileWings
        collapse={{ title: 'Совместный доступ', trailing: inviteButton }}
      />

      <PageContent>
        <HubCollapseAnchor>
          <div className="flex items-center justify-between pr-3.5 pl-6">
            <HubTitle className="pl-0">Совместный доступ</HubTitle>
            {inviteButton}
          </div>
        </HubCollapseAnchor>

        {summaryQuery.isPending ? (
          <div className="mt-6">
            <ParticipantsHubSkeleton />
          </div>
        ) : summaryQuery.isError ? (
          <ErrorCard
            title="Не удалось загрузить раздел"
            onRetry={() => void summaryQuery.refetch()}
            className="mt-6"
          />
        ) : (
          <div className="mt-6 flex flex-col gap-4 px-6">
            <HubCard
              href={ROUTES.participantsList}
              title="Ваши участники"
              description="Приглашенные пользователи, у которых есть доступ к вашим объектам"
              image="/images/tariff/tariff-about-sharing.png"
              CountIcon={Team}
              countLabel={participantsCountLabel(summaryQuery.data.participantsCount)}
            />
            <HubCard
              href={ROUTES.participantsProperties}
              title="Объекты пользователей"
              description="Объекты пользователей, к которым у вас есть доступ"
              image="/images/tariff/tariff-about-objects.png"
              CountIcon={Objects}
              countLabel={sharedPropertiesCountLabel(summaryQuery.data.accessiblePropertiesCount)}
            />
          </div>
        )}
      </PageContent>

      <StickyBottomBar>
        <Button className="w-full" onClick={() => router.push(ROUTES.participantsInvite)}>
          Пригласить участника
        </Button>
      </StickyBottomBar>

      {invitedPopup && (
        <ParticipantSuccessPopup
          title="Участник приглашен"
          onClose={() => popParticipantPopup()}
        />
      )}
    </>
  );
}

/** Карточка-переход хаба (макет 2008-47013): серая 32/32 радиус 32, сверху
 * заголовок 20/24 + описание 14/16 и 3D-иллюстрация 48, снизу ряд счётчика
 * (иконка 24 + подпись 16/18) и шеврон. Целиком — ссылка на раздел. */
function HubCard({
  href,
  title,
  description,
  image,
  CountIcon,
  countLabel,
}: {
  readonly href: string;
  readonly title: string;
  readonly description: string;
  readonly image: string;
  readonly CountIcon: ComponentType<{ className?: string }>;
  readonly countLabel: string;
}): JSX.Element {
  return (
    <Link
      href={href}
      className="flex flex-col gap-6 rounded-[32px] bg-surface-muted p-8 outline-none transition-opacity hover:opacity-80 active:opacity-80 focus-visible:ring-4 focus-visible:ring-primary"
    >
      <span className="flex items-start gap-6">
        <span className="flex min-w-0 flex-1 flex-col gap-2">
          <h2 className="m-0 text-xl font-semibold leading-6 text-content">{title}</h2>
          <span className="text-sm leading-4 text-content-secondary">{description}</span>
        </span>
        <Image src={image} alt="" width={48} height={48} className="h-12 w-12 shrink-0" />
      </span>
      <span className="flex items-center justify-between">
        <span className="flex items-center gap-2 text-content">
          <CountIcon className="h-6 w-6" />
          <span className="text-base font-medium leading-[18px]">{countLabel}</span>
        </span>
        <SmallArrowRight className="h-6 w-6 text-content-tertiary" />
      </span>
    </Link>
  );
}
