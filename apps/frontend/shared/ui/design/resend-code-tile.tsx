import type { JSX } from 'react';
import { formatCountdown } from '@/shared/lib/countdown';
import { cn } from '@/shared/lib/cn';
import { Button } from './button';

/**
 * Плитка повторной отправки кода — resend-канон шага кода (решение
 * владельца 17.09, карта #723/#733; Figma 1869-68137 — таймер идёт,
 * 2343-51004 — таймер истёк). Кнопка Secondary «Отправить новый код»:
 * disabled, пока идёт таймер, с подписью «Запросить новый код можно /
 * через ММ:СС» — две строки, перенос после «можно» (Roboto Mono 14/16,
 * tertiary, по центру; зазор кнопка — подпись 16px); по истечении подпись
 * скрыта, кнопка активна. Пропы remainingSeconds/loading/onResend; где
 * хранить дедлайн таймера канон не диктует (стратегия — за фичей; здесь
 * useCountdown). Потребители — шаги кода смены телефона и почты; логин
 * переехал на канон в карте редизайна авторизации #761 (шаг кода, #765).
 */

export type ResendCodeTileProps = {
  /** Остаток таймера в секундах: > 0 — плитка disabled с подписью, 0 —
   * активна без подписи. */
  readonly remainingSeconds: number;
  readonly loading?: boolean;
  readonly onResend: () => void;
  readonly className?: string;
};

export function ResendCodeTile({
  remainingSeconds,
  loading = false,
  onResend,
  className,
}: ResendCodeTileProps): JSX.Element {
  const isWaiting = remainingSeconds > 0;

  return (
    <div className={cn('flex w-full flex-col gap-4', className)}>
      <Button
        variant="secondary"
        className="w-full"
        loading={loading}
        // loading дублируется в disabled: Button резолвит disabled ?? loading,
        // и явный disabled без || не даёт loading диспейблить кнопку —
        // «на время отправки» оставался активной (#1099).
        disabled={isWaiting || loading}
        onClick={onResend}
      >
        Отправить новый код
      </Button>
      {isWaiting && (
        <p className="m-0 text-center font-mono text-sm font-medium leading-4 text-content-tertiary">
          {/* Перенос после «можно» — из макетов: подпись канона всегда
              двухстрочная (1869:68137, логин 2349:67383/67396/67411 —
              высота слоя 32, сверка Т5 #1102). {' '} держит пробел в
              textContent: без него <br/> склеивает «можно» и «через» —
              локаторы текста и скринридеры теряют фразу. */}
          Запросить новый код можно{' '}
          <br />
          через {formatCountdown(remainingSeconds)}
        </p>
      )}
    </div>
  );
}
