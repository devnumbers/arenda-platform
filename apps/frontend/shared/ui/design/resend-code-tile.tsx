import type { JSX } from 'react';
import { formatCountdown } from '@/shared/lib/countdown';
import { cn } from '@/shared/lib/cn';
import { Button } from './button';

/**
 * Плитка повторной отправки кода — resend-канон шага кода (решение
 * владельца 17.09, карта #723/#733; Figma 1869-68137 — таймер идёт,
 * 2343-51004 — таймер истёк). Кнопка Secondary «Отправить новый код»:
 * disabled, пока идёт таймер, с подписью «Запросить новый код можно
 * через ММ:СС» (Roboto Mono 14/16, tertiary, по центру; зазор кнопка —
 * подпись 16px); по истечении подпись скрыта, кнопка активна. Пропы
 * remainingSeconds/loading/onResend; где хранить дедлайн таймера канон
 * не диктует (стратегия — за фичей; здесь useCountdown). Потребители —
 * шаги кода смены телефона и почты; логин переехал на канон в карте
 * редизайна авторизации #761 (шаг кода, #765).
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
        disabled={isWaiting}
        onClick={onResend}
      >
        Отправить новый код
      </Button>
      {isWaiting && (
        <p className="m-0 text-center font-mono text-sm font-medium leading-4 text-content-tertiary">
          Запросить новый код можно через {formatCountdown(remainingSeconds)}
        </p>
      )}
    </div>
  );
}
