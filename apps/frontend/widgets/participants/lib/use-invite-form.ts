'use client';

import { useState } from 'react';

import { isEmailValid } from '@/shared/lib/email';
import type { ParticipantAccessRole } from '@/entities/participants';
import {
  useInviteParticipant,
  type InviteParticipantCommand,
  type InviteParticipantResult,
} from '@/features/participants';
import type { ApiError } from '@/shared/api/errors';

/** Подпись поля почты при провале валидации — общая для форм
 * приглашения. */
export const INVITE_EMAIL_ERROR = 'Укажите корректную электронную почту';

/** Инфраструктурная ошибка (не семантический 400) — общая фраза. */
export const INVITE_GENERIC_ERROR = 'Не удалось пригласить — попробуйте еще раз';

/** Семантические 400 (#694: свой email, некорректная почта) бэк
 * объясняет по-человечески — показываем его текст; остальное —
 * инфраструктура, общая фраза. */
export function inviteServerError(error: ApiError): string {
  return error.status === 400 && error.detail.length > 0
    ? error.detail
    : INVITE_GENERIC_ERROR;
}

/** Хук формы приглашения — общий для приглашения хаба (#699) и экрана
 * «Пригласите участника» от объекта (#700,
 * property-participants-invite-screen): email + роль, обе ошибки с общим
 * сбросом при наборе, валидация почты и семантика ответа партии
 * (#694): granted = 0 — детерминированный исход, повтор не поможет,
 * поэтому без «попробуйте еще раз» (решение владельца 17.09) —
 * сообщение задаёт потребитель. */
export function useInviteForm({
  propertyIds,
  grantedZeroMessage,
  onInvited,
}: {
  readonly propertyIds: () => readonly string[];
  readonly grantedZeroMessage: string;
  readonly onInvited: () => void;
}): {
  readonly email: string;
  readonly setEmailValue: (value: string) => void;
  readonly role: ParticipantAccessRole;
  readonly setRole: (role: ParticipantAccessRole) => void;
  readonly emailError: string | null;
  readonly serverError: string | null;
  readonly isPending: boolean;
  readonly submitInvite: () => void;
} {
  const invite = useInviteParticipant();
  const [email, setEmail] = useState('');
  const [role, setRole] = useState<ParticipantAccessRole>('viewer');
  const [emailError, setEmailError] = useState<string | null>(null);
  const [serverError, setServerError] = useState<string | null>(null);

  const setEmailValue = (value: string): void => {
    setEmail(value);
    setEmailError(null);
    setServerError(null);
  };

  const submitInvite = (): void => {
    const trimmedEmail = email.trim();
    if (!isEmailValid(trimmedEmail)) {
      setEmailError(INVITE_EMAIL_ERROR);
      return;
    }
    invite.mutate(
      { email: trimmedEmail, role, propertyIds: propertyIds() } satisfies InviteParticipantCommand,
      {
        onSuccess: (result: InviteParticipantResult) => {
          if (result.granted === 0) {
            setServerError(grantedZeroMessage);
            return;
          }
          onInvited();
        },
        onError: (error: ApiError) => {
          setServerError(inviteServerError(error));
        },
      },
    );
  };

  return {
    email,
    setEmailValue,
    role,
    setRole,
    emailError,
    serverError,
    isPending: invite.isPending,
    submitInvite,
  };
}
