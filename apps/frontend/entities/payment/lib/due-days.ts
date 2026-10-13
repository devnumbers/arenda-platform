/**
 * Группы секции «На оплату» по реальным дням (#1347, решение владельца
 * 13.10 — ревизия прежних бакетов 2..3/4..7 из research #1343: склейка
 * разных дат под одним заголовком читалась как ошибка, каждая дата — своя
 * группа). Сервер текстов не строит (канон «серверные тексты не строим») —
 * заголовок группы выводится из daysLeft, порядок строк внутри дня
 * группировка сохраняет; чип страницы разворачивает порядок групп целиком.
 */

import type { PaymentDueItem } from '../model/types';
import { pluralize } from '@/shared/lib/pluralize';

/** Заголовок реального дня окна (и метка карточки хаба): «Сегодня» /
 * «Завтра» / «Через N дней» со склонением канона pluralize; за границами
 * окна 0..7 не бывает — крайний остаток печатается как есть. */
export function dueDaysLabel(daysLeft: number): string {
  if (daysLeft <= 0) {
    return 'Сегодня';
  }
  if (daysLeft === 1) {
    return 'Завтра';
  }
  return `Через ${daysLeft} ${pluralize(daysLeft, 'день', 'дня', 'дней')}`;
}

/** Одна группа страницы: один реальный день окна, непустая по построению. */
export type DueDayGroup = {
  readonly daysLeft: number;
  readonly label: string;
  readonly items: ReadonlyArray<PaymentDueItem>;
};

/** Группировка строк страницы по реальным дням: каждый встретившийся
 * daysLeft — своя группа; группы идут по возрастанию дней (порядок входа
 * не важен), порядок строк внутри дня сохраняется. */
export function groupDueByDay(
  items: ReadonlyArray<PaymentDueItem>,
): ReadonlyArray<DueDayGroup> {
  const byDay = new Map<number, PaymentDueItem[]>();
  for (const item of items) {
    const group = byDay.get(item.daysLeft);
    if (group) {
      group.push(item);
    } else {
      byDay.set(item.daysLeft, [item]);
    }
  }
  return [...byDay.entries()]
    .sort(([left], [right]) => left - right)
    .map(([daysLeft, itemsOfDay]) => ({
      daysLeft,
      label: dueDaysLabel(daysLeft),
      items: itemsOfDay,
    }));
}
