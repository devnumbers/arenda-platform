/**
 * Бакеты секции «На оплату» (#1346): группировка строк по daysLeft с
 * фиксированным порядком и каноническими заголовками (контракт по
 * research-тикету #1343). Сервер текстов не строит (канон «серверные
 * тексты не строим») — ключи бакетов структурные, строки приходят из
 * GET /payments/due уже в порядке бакетов (daysLeft asc) и «Сначала
 * новые» внутри (dueDate desc, operationId desc): группировка сохраняет
 * порядок входа.
 */

import type { DueBucket, PaymentDueItem } from '../model/types';

/** Порядок бакетов секции — фиксированный контрактом. */
export const DUE_BUCKET_ORDER: ReadonlyArray<DueBucket> = ['today', 'tomorrow', 'in3days', 'in7days'];

/** Канонические заголовки бакетов (research #1343: промежуточные остатки
 * 2..3 и 4..7 прижимаются к ближнему заголовку). */
export const DUE_BUCKET_LABELS: Readonly<Record<DueBucket, string>> = {
  today: 'Сегодня',
  tomorrow: 'Завтра',
  in3days: 'Через 3 дня',
  in7days: 'Через 7 дней',
};

/** Бакет строки по остатку дней: 0 → «Сегодня», 1 → «Завтра», 2..3 →
 * «Через 3 дня», 4..7 → «Через 7 дней». За границами 0..7 (не бывает по
 * построению окна) — ближний заголовок, чтобы экстремальный остаток не
 * разваливал секцию. */
export function dueBucketOf(daysLeft: number): DueBucket {
  if (daysLeft <= 0) return 'today';
  if (daysLeft === 1) return 'tomorrow';
  if (daysLeft <= 3) return 'in3days';
  return 'in7days';
}

/** Один бакет группировки: пустые бакеты секция не показывает. */
export type DueBucketGroup = {
  readonly bucket: DueBucket;
  readonly label: string;
  readonly items: ReadonlyArray<PaymentDueItem>;
};

/** Группировка строк секции в непустые бакеты в фиксированном порядке.
 * Вход — ответ сервера (уже отсортирован), порядок внутри бакета
 * сохраняется. */
export function groupDueByBucket(items: ReadonlyArray<PaymentDueItem>): ReadonlyArray<DueBucketGroup> {
  const byBucket = new Map<DueBucket, PaymentDueItem[]>();
  for (const item of items) {
    const bucket = dueBucketOf(item.daysLeft);
    const group = byBucket.get(bucket);
    if (group) {
      group.push(item);
    } else {
      byBucket.set(bucket, [item]);
    }
  }
  return DUE_BUCKET_ORDER.filter((bucket) => byBucket.has(bucket)).map((bucket) => ({
    bucket,
    label: DUE_BUCKET_LABELS[bucket],
    items: byBucket.get(bucket) ?? [],
  }));
}
