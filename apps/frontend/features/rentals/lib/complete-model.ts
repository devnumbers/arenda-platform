import { cmp, fullMonthsBetween, type IsoDate } from '@/shared/lib/calendar';
import { parseRublesToKopecks } from '@/shared/lib/format-money';
import { pluralize } from '@/shared/lib/pluralize';
import type { Rental, RentalCompleteCommand } from '@/entities/rental';

/** Черновик мастера завершения (#534): «сырое» поле суммы залога (как в
 * формах аренды) и комментарий возврата. Форма-состояние сценария — живёт
 * в фиче, как RentalEditForm и RentalWizardDraft. */
export type RentalCompleteDraft = {
  readonly completedDate: IsoDate | undefined;
  readonly depositRaw: string;
  readonly comment: string;
};

/**
 * Мастер «Завершение аренды» (#534): чистые правила сборки команды
 * POST …/complete (ADR 0053 §3 — дата в [начало, сегодня], «По плану»
 * подставляет клиент) и строки экрана итогов. Деньги — копейки через
 * канонный parseRublesToKopecks; 0 валиден — «не вернул».
 */

function yearsWord(count: number): string {
  return pluralize(count, 'год', 'года', 'лет');
}

function monthsWord(count: number): string {
  return pluralize(count, 'месяц', 'месяца', 'месяцев');
}

/** Срок аренды для итогов (Figma 1433:61927 «2 года, 8 месяцев»): полные
 * календарные месяцы между началом и завершением. Неполный месяц —
 * «Меньше месяца» (нулевой срок в макет не пойман — сверить на приёмке). */
export function rentalDurationLine(startDate: IsoDate, endDate: IsoDate): string {
  const months = fullMonthsBetween(startDate, endDate);
  if (months <= 0) {
    return 'Меньше месяца';
  }
  const years = Math.floor(months / 12);
  const rest = months % 12;
  if (years === 0) {
    return `${rest} ${monthsWord(rest)}`;
  }
  if (rest === 0) {
    return `${years} ${yearsWord(years)}`;
  }
  return `${years} ${yearsWord(years)}, ${rest} ${monthsWord(rest)}`;
}

/** Дата планового окончания, доступная чипу «По плану»: у срочной аренды
 * наступивший план (≤ сегодня — будущим числом завершать нельзя, ADR 0053);
 * у бессрочной и не наступившего плана подсказки нет. */
export function completePlannedEndDate(rental: Rental): IsoDate | undefined {
  const planned = rental.plannedEndDate;
  if (planned === null || cmp(planned, rental.today) > 0) {
    return undefined;
  }
  return planned;
}

/** Команда завершения из черновика мастера: сумма залога парсится из
 * «сырого» поля (пустое/нечитаемое — команды нет, шаг не готов), комментарий
 * триммится и не пустой — только тогда попадает в тело (контракт: комментарий
 * только при сумме; сумма в мастере обязательна). */
export function buildRentalCompleteCommand(
  draft: RentalCompleteDraft,
): RentalCompleteCommand | undefined {
  if (draft.completedDate === undefined) {
    return undefined;
  }
  const amountKopecks = parseRublesToKopecks(draft.depositRaw);
  if (amountKopecks === undefined) {
    return undefined;
  }
  const comment = draft.comment.trim();
  return {
    completedDate: draft.completedDate,
    depositReturn: comment.length > 0
      ? { amountKopecks, comment }
      : { amountKopecks },
  };
}
