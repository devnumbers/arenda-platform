'use client';

import { useCallback, useState, type ChangeEvent } from 'react';
import type { ApiError } from '@/shared/api/errors';
import { useCountdown } from '@/shared/lib/hooks/use-countdown';
import { loginCodeFromInput } from '@/shared/lib/login-code';
import { codeFieldError, invalidCodeDetail, RESEND_COOLDOWN_MS } from './code-step';

/** Состояние одного шага кода resend-канона (#733): маскированный ввод,
 * inline-ошибка 401, дедлайн таймера и производная ошибка поля. Общий у
 * флоу флаг isSubmitAttempted остаётся снаружи (в email-флоу его делит
 * с шагом адреса) — хук получает его значение и сброс. Мутации и тосты —
 * тоже снаружи: шагу неважно, какой send/verify вызов его перезаряжает. */
export type UseCodeStepOptions = {
  /** Общий флаг «сабмит уже пытались» из флоу — управляет валидационной
   * ошибкой поля. */
  isSubmitAttempted: boolean;
  /** Сброс общего флага флоу (крест-очистка, «Назад», новая отправка). */
  onAttemptReset: () => void;
};

export type CodeStepController = {
  /** Значение поля кода (маска 6 цифр). */
  readonly value: string;
  /** Ошибка error-проп поля: валидация неполного кода либо inline 401. */
  readonly error: string | undefined;
  /** Остаток resend-таймера в секундах (0 — плитка разблокирована). */
  readonly remainingSeconds: number;
  /** onChange поля: маска ввода, inline-ошибка вводом сбрасывается. */
  readonly handleChange: (event: ChangeEvent<HTMLInputElement>) => void;
  /** Черновик шага уходит (крест-очистка, «Назад»): значение и ошибки
   * сбрасываются, дедлайн таймера не трогается — таймер флоу живёт (#733). */
  readonly clear: () => void;
  /** Только старт/перезапуск таймера от последней доставки — автопуск кода
   * шага 1 email-флоу: поле и флаги в этот момент не трогаются. */
  readonly restartCooldown: () => void;
  /** Код (пере)выпущен: чистое поле без ошибок и таймер с нуля — переход
   * на шаг кода и onSuccess resend-плитки. */
  readonly rearm: () => void;
  /** Ошибка verify-мутации: 401 — inline в поле, остальное — тост сценария. */
  readonly routeVerifyError: (error: ApiError, toastScenario: (error: ApiError) => void) => void;
};

export function useCodeStep({ isSubmitAttempted, onAttemptReset }: UseCodeStepOptions): CodeStepController {
  const [value, setValue] = useState('');
  const [inlineError, setInlineError] = useState<string | null>(null);
  const [deadline, setDeadline] = useState<number | null>(null);
  const remainingSeconds = useCountdown(deadline);

  const handleChange = useCallback((event: ChangeEvent<HTMLInputElement>): void => {
    setValue(loginCodeFromInput(event.currentTarget.value));
    setInlineError(null);
  }, []);

  const clear = useCallback((): void => {
    setValue('');
    setInlineError(null);
    onAttemptReset();
  }, [onAttemptReset]);

  const restartCooldown = useCallback((): void => {
    setDeadline(Date.now() + RESEND_COOLDOWN_MS);
  }, []);

  const rearm = useCallback((): void => {
    setValue('');
    setInlineError(null);
    setDeadline(Date.now() + RESEND_COOLDOWN_MS);
    onAttemptReset();
  }, [onAttemptReset]);

  const routeVerifyError = useCallback(
    (error: ApiError, toastScenario: (error: ApiError) => void): void => {
      const inline = invalidCodeDetail(error);
      if (inline !== null) {
        setInlineError(inline);
        return;
      }
      toastScenario(error);
    },
    [],
  );

  return {
    value,
    error: codeFieldError(isSubmitAttempted, value, inlineError),
    remainingSeconds,
    handleChange,
    clear,
    restartCooldown,
    rearm,
    routeVerifyError,
  };
}
