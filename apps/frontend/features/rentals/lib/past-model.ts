import { fullMonthsBetween, inclusiveDays, type IsoDate } from '@/shared/lib/calendar';
import { daysWord } from '@/shared/lib/days-word';
import { formatDayMonthWithYear } from '@/shared/lib/date-format';
import { monthsWord } from '@/shared/lib/months-word';
import type { Rental } from '@/entities/rental';
import { rentAmountPerMonth, type RentalTermsRow } from './rental-view';

/**
 * Чистый рендер-слой «Прошлых аренд» (#535, Figma 1302:52462, 1550:94517):
 * карточки списка завершённых, заголовок детализации («24 месяца» либо
 * «Не было платежей»), строки карточки и порядковые номера платежей
 * истории. Только правила текстов — экраны и данные в слоях выше.
 */

/** Завершённые аренды списка «Прошлые аренды»: сервер кладёт их после
 * незавершённой по дате завершения, свежие сверху (ADR 0053 §4) —
 * фильтр порядок сохраняет. */
export function completedRentalsOf(items: ReadonlyArray<Rental>): ReadonlyArray<Rental> {
  return items.filter((rental) => rental.status === 'completed');
}

/** Чем аренда кончилась: доменный факт завершения, для незавершённой —
 * плановое окончание (оба nullable по типу); совсем без конца — начало
 * (ноль прожитого). */
function rentalEndDate(rental: Rental): IsoDate | null {
  return rental.completedDate ?? rental.plannedEndDate;
}

/** Конец для сроковых строк: тот же конец, а без него — начало
 * (inclusiveDays даст 1 — минимальный срок). */
function completedRentalEnd(rental: Rental): IsoDate {
  return rentalEndDate(rental) ?? rental.startDate;
}

/** Прожитые полные месяцы: от начала до фактической даты завершения;
 * та не задана (не бывает у завершённой, но тип nullable) — плановое
 * окончание, затем начало (ноль). */
export function completedRentalMonths(rental: Rental): number {
  return fullMonthsBetween(rental.startDate, completedRentalEnd(rental));
}

/** Заголовок карточки списка: срок в месяцах — «24 месяца»; неполный
 * месяц — точные дни включительно («16 дней», «1 день»; решение владельца
 * 22.10 — вместо «Меньше месяца», как срок итогов #534). Конец не раньше
 * начала гарантирован контрактами завершения (ADR 0053 §3). */
export function pastRentalCardTitle(rental: Rental): string {
  const months = completedRentalMonths(rental);
  if (months <= 0) {
    const days = inclusiveDays(rental.startDate, completedRentalEnd(rental));
    return `${days} ${daysWord(days)}`;
  }
  return `${months} ${monthsWord(months)}`;
}

/** Заголовок детализации (H1): без оплаченных операций — «Не было
 * платежей» (1550:94517), иначе срок карточки. */
export function pastRentalTitle(rental: Rental, paidOperationsCount: number): string {
  if (paidOperationsCount === 0) {
    return 'Не было платежей';
  }
  return pastRentalCardTitle(rental);
}

/** Окончание на карточке — доменный факт завершения (решение F2 аудита
 * #801): завершённая «кончилась» датой завершения, плановая остаётся
 * видимой в «Условиях аренды» завершённой. Факта нет (тип nullable,
 * живьём не бывает) — плановая; нет и её — «Не указано» (1550:94804). */
function pastRentalEndDate(rental: Rental): string {
  const end = rentalEndDate(rental);
  return end === null ? 'Не указано' : formatDayMonthWithYear(end, rental.today);
}

/** Строки карточки списка (1302:52462): плата, начало, окончание —
 * формат списковых дат; окончание — фактическая дата завершения
 * (F2 #801). */
export function pastRentalRows(rental: Rental): ReadonlyArray<RentalTermsRow> {
  return [
    { label: 'Арендная плата', value: rentAmountPerMonth(rental.rentPayment.amountKopecks) },
    {
      label: 'Начало аренды',
      value: formatDayMonthWithYear(rental.startDate, rental.today),
    },
    {
      label: 'Окончание аренды',
      value: pastRentalEndDate(rental),
    },
  ];
}
