'use client';

import { useState, type JSX } from 'react';
import Image from 'next/image';
import { useParams, useRouter } from 'next/navigation';
import { goBack } from '@/shared/lib/navigation';
import { ROUTES } from '@/shared/config/routes';
import { isEmailValid } from '@/shared/lib/email';
import type { ParticipantAccessRole } from '@/entities/participants';
import { useInviteParticipant } from '@/features/participants';
import {
  Button,
  PageContent,
  StickyBottomBar,
  TextField,
  TopNav,
  TopNavBackButton,
} from '@/shared/ui/design';
import { stageParticipantPopup } from '../lib/participant-popups';
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

  const invite = useInviteParticipant();

  const [email, setEmail] = useState('');
  const [role, setRole] = useState<ParticipantAccessRole>('viewer');
  const [emailError, setEmailError] = useState<string | null>(null);
  const [serverError, setServerError] = useState<string | null>(null);

  const trimmedEmail = email.trim();
  const canSubmit = trimmedEmail.length > 0 && !invite.isPending;

  const setEmailValue = (value: string): void => {
    setEmail(value);
    setEmailError(null);
    setServerError(null);
  };

  const submitInvite = (): void => {
    if (!isEmailValid(trimmedEmail)) {
      setEmailError('Укажите корректную электронную почту');
      return;
    }
    invite.mutate(
      { email: trimmedEmail, role, propertyIds: [propertyId] },
      {
        onSuccess: (result) => {
          if (result.granted === 0) {
            // Детерминированный исход (доступ/приглашение уже есть):
            // повтор не поможет — без «попробуйте еще раз» (решение #699).
            setServerError('Пользователь уже имеет доступ к объекту');
            return;
          }
          stageParticipantPopup('invited');
          goBack(router, ROUTES.propertyParticipants(propertyId));
        },
        onError: (error) => {
          // Семантические 400 (#694: свой email, некорректная почта) бэк
          // объясняет по-человечески — показываем его текст; остальное —
          // инфраструктура, общая фраза.
          setServerError(
            error.status === 400 && error.detail.length > 0
              ? error.detail
              : 'Не удалось пригласить — попробуйте еще раз',
          );
        },
      },
    );
  };

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
      <PageContent className="px-6">
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
              disabled={invite.isPending}
              onChange={setRole}
            />
          </div>
        </div>
      </PageContent>

      <StickyBottomBar>
        <Button
          className="w-full"
          disabled={!canSubmit}
          loading={invite.isPending}
          onClick={submitInvite}
        >
          Пригласить
        </Button>
      </StickyBottomBar>
    </>
  );
}
