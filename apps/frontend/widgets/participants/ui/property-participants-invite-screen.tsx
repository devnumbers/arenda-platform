'use client';

import type { JSX } from 'react';
import Image from 'next/image';
import { useParams, useRouter } from 'next/navigation';
import { goBack } from '@/shared/lib/navigation';
import { ROUTES } from '@/shared/config/routes';
import {
  Button,
  PageContent,
  StickyBottomBar,
  TextField,
  TopNav,
  TopNavBackButton,
} from '@/shared/ui/design';
import { stageParticipantPopup } from '../lib/participant-popups';
import { useInviteForm } from '../lib/use-invite-form';
import { ParticipantRoleSegmented } from './participant-role-segmented';

/**
 * Экран «Пригласите участника» от объекта (карта #692, тикет #700;
 * Figma 1978-103001, 2035-82097): тот же флоу приглашения хаба (#699),
 * но объект один — свёрнутого выбора объектов и пикера нет, копия
 * описания макета («Мы отправим приглашение в ваш объект»).
 *
 * Отправка — POST /participants/invite (#694) с одним объектом:
 * зарегистрированная почта получает членство сразу (слот получателя может
 * сделать его suspended — это приглашённый исход, попап общий, решение
 * владельца 17.09), незарегистрированная — pending-приглашение и письмо.
 * granted=0 — объект ушёл в skipped_* (обычно доступ уже есть): честный
 * текст без «попробуйте еще раз» (решение #699, единственное число).
 * Успех — попап «Участник приглашен» (2035-82202) на списке участников
 * объекта: goBack по канону истории, staged-флаг рендерит приёмник.
 */
export function PropertyParticipantsInviteScreen(): JSX.Element {
  const params = useParams<{ id: string }>();
  const propertyId = params.id;
  const router = useRouter();

  const form = useInviteForm({
    propertyIds: () => [propertyId],
    grantedZeroMessage: 'Пользователь уже имеет доступ к объекту',
    onInvited: () => {
      stageParticipantPopup('invited');
      goBack(router, ROUTES.propertyParticipants(propertyId));
    },
  });
  const { email, setEmailValue, role, setRole, emailError, serverError, submitInvite } = form;

  const trimmedEmail = email.trim();
  const canSubmit = trimmedEmail.length > 0 && !form.isPending;

  return (
    <>
      <TopNav
        leading={
          <TopNavBackButton fallbackHref={ROUTES.propertyParticipants(propertyId)} />
        }
      />

      {/* Боковой отступ 24px по макету (урок приёмки #699) — на все
       * состояния. Данные не грузятся (объект фиксирован) — скелетона и
       * error-состояния загрузки нет; ошибка мутации живёт под полем. */}
      {/* Клиренс футера — паттерн «CTA над TabBar» (#1166). */}
      <PageContent className="px-6" aboveTabBarFooter>
        <div className="flex flex-col pt-4">
          <Image
            src="/images/tariff/tariff-about-sharing.png"
            alt=""
            width={96}
            height={96}
            className="h-24 w-24 self-center"
          />
          <h1 className="mt-8 text-[28px] font-semibold leading-8 text-content">
            Пригласите участника
          </h1>
          <p className="mt-2 text-sm leading-4 text-content-secondary">
            Укажите электронную почту и выберите роль — просмотр или
            редактирование. Мы отправим приглашение в ваш объект
          </p>

          <div className="mt-8">
            <TextField
              variant="titleIn"
              title="Электронная почта"
              type="email"
              autoComplete="email"
              value={email}
              error={emailError ?? serverError ?? undefined}
              onClear={() => setEmailValue('')}
              onChange={(event) => setEmailValue(event.target.value)}
            />
          </div>

          <div className="mt-8">
            <ParticipantRoleSegmented
              value={role}
              disabled={form.isPending}
              onChange={setRole}
            />
          </div>
        </div>
      </PageContent>

      {/* CTA над видимым нижним меню (решение владельца 06.10.2026, #1166). */}
      <StickyBottomBar aboveTabBar>
        <Button
          className="w-full"
          disabled={!canSubmit}
          loading={form.isPending}
          onClick={submitInvite}
        >
          Пригласить
        </Button>
      </StickyBottomBar>
    </>
  );
}
