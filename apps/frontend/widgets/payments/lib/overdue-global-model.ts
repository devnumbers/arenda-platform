/**
 * Модель страницы «Просроченные платежи» (карта #573, тикет #580; макеты
 * 706:14684/885:18755): правила с накопленной просрочкой целого видимого
 * скоупа из фида #575, сортировка чипом по возрасту просрочки — «Новые»
 * (недавно просроченные первыми, дефолт) / «Старые».
 */

import type { GlobalPayment } from '@/entities/payment';

/** Направление сортировки страницы: по возрасту просрочки. */
export type OverdueSort = 'new' | 'old';

/** Разбор ?sort= строки страницы (конвенция книги контактов):
 * неизвестное и отсутствующее значения — дефолт «Новые». */
export function parseOverdueSortParams(
  sort: string | string[] | undefined,
): OverdueSort {
  return typeof sort === 'string' && sort === 'old' ? 'old' : 'new';
}

/** Просроченные правила скоупа в направлении сортировки страницы.
 * Возраст — overdueDays фида (возраст старейшего вхождения, считает
 * сервер по календарю собственника, ADR 0048); аномальный null (фид
 * гарантирует возраст при накопленной просрочке) уходит в конец, равный
 * возраст сохраняет порядок фида (сортировка стабильна). */
export function globalOverdueList(
  items: ReadonlyArray<GlobalPayment>,
  sort: OverdueSort,
): ReadonlyArray<GlobalPayment> {
  const overdue = items.filter((item) => item.overdueOperationCount > 0);
  return overdue.sort((a, b) => {
    const ageA = a.overdueDays;
    const ageB = b.overdueDays;
    // Аномальный null (фид гарантирует возраст при накопленной
    // просрочке) — в конец в обоих направлениях.
    if (ageA === null || ageB === null) {
      if (ageA === ageB) return 0;
      return ageA === null ? 1 : -1;
    }
    return sort === 'old' ? ageB - ageA : ageA - ageB;
  });
}
