/**
 * Обратный отсчёт подписи-таймера resend-канона шага кода (#733, макет
 * 1869-68137 «Запросить новый код можно через 00:59»): остаток секунд от
 * дедлайна и формат ММ:СС. Хранение дедлайна — за потребителем (канон
 * плитки его не диктует); тикающий остаток — useCountdown.
 */

/** Показанный остаток: полные секунды до дедлайна (epoch ms), не считая
 * самой секунды дедлайна — пуск на 60 с рисует «00:59», как в макете (и
 * разблокирует плитку на ≤1 с раньше серверного троттлинга — см.
 * widgets/profile/lib/code-step.ts). Нет дедлайна или он прошёл — 0
 * (плитка разблокирована). */
export function remainingSecondsUntil(deadlineMs: number | null, nowMs: number): number {
  if (deadlineMs === null) {
    return 0;
  }
  return Math.max(0, Math.ceil((deadlineMs - nowMs) / 1000) - 1);
}

/** Остаток как «ММ:СС» с ведущими нулями (подпись-таймер макета). */
export function formatCountdown(totalSeconds: number): string {
  const mm = String(Math.floor(totalSeconds / 60)).padStart(2, '0');
  const ss = String(totalSeconds % 60).padStart(2, '0');
  return `${mm}:${ss}`;
}
