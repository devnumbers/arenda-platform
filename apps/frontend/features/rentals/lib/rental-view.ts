import type { IsoDate } from '@/shared/lib/calendar';
import { fullMonthsBetween } from '@/shared/lib/calendar';
import { formatDayMonth, formatDayMonthWithYear, formatOverdueDays } from '@/shared/lib/date-format';
import { formatMoneyKopecks, ratioToPercent } from '@/shared/lib/format-money';
import { pluralize } from '@/shared/lib/pluralize';
import type {
  Rental,
  RentalNextPayment,
  RentalProgress,
  RentalTenant,
} from '@/entities/rental';
import { paymentDayLabel, utilitiesLabel } from './wizard-model';

/**
 * Чистый рендер-слой детализации и условий аренды (#531, Figma 1232:61291,
 * 1302:53783/1550:94419): заголовок «Оплачено N из M месяцев», подписи
 * карточки прогресса, строки условий с «Не указано» для пустых. Только
 * правила текстов — экраны и данные в слоях выше.
 */

/** Строка условий «метка — значение» (Figma EL-c094fdcc: 12px между колонками). */
export type RentalTermsRow = {
  readonly label: string;
  readonly value: string;
};

function monthsWord(count: number): string {
  return pluralize(count, 'месяц', 'месяца', 'месяцев');
}

/** Существительное после «из N» — родительный падеж: «из 24 месяцев»,
 * «из 21 месяца» (макет 1232:61492); единственное число — у 1, 21, 31…
 * (11, 111 — множительное). */
function monthsFromWord(count: number): string {
  const singularGenitive = count % 10 === 1 && count % 100 !== 11;
  return singularGenitive ? 'месяца' : 'месяцев';
}

/** Заголовок детализации: у срочной — «N из M месяцев» (родительный по
 * итогу), у бессрочной тотал отсутствует — только оплаченные месяцы. */
export function rentalPaidTitle(progress: RentalProgress): string {
  const { paidMonths, totalMonths } = progress;
  if (totalMonths === null) {
    return `${paidMonths} ${monthsWord(paidMonths)}`;
  }
  return `${paidMonths} из ${totalMonths} ${monthsFromWord(totalMonths)}`;
}

/** Процент прогресс-бара: доля оплаченных месяцев; у бессрочной бара нет. */
export function rentalProgressPercent(progress: RentalProgress): number | null {
  const { paidMonths, totalMonths } = progress;
  if (totalMonths === null || totalMonths <= 0) {
    return null;
  }
  const percent = Math.round(ratioToPercent(paidMonths / totalMonths));
  return Math.min(100, Math.max(0, percent));
}

/** Подпись у иконки календаря (Figma 1550:92676 — «150 дней до следующего
 * платежа»); день наступил — «Платёж сегодня». */
export function rentalNextPaymentLine(nextPayment: RentalNextPayment): string {
  if (nextPayment.daysUntil <= 0) {
    return 'Платёж сегодня';
  }
  return `${formatOverdueDays(nextPayment.daysUntil)} до следующего платежа`;
}

/** Строка «Осталось 23 месяца аренды»; у бессрочной остатка нет — строки нет.
 * Глагол согласуется числом: «Остался 21 месяц», но «Осталось 11 месяцев». */
export function rentalRemainingLine(monthsRemaining: number | null): string | undefined {
  if (monthsRemaining === null) {
    return undefined;
  }
  const singular = monthsRemaining % 10 === 1 && monthsRemaining % 100 !== 11;
  const remained = singular ? 'Остался' : 'Осталось';
  return `${remained} ${monthsRemaining} ${monthsWord(monthsRemaining)} аренды`;
}

/** Карточка прогресса на детализации видима, пока ей есть что жить: со
 * следующим платежом — строка дней и бар (1232:61259), без него — строка
 * остатка или прошедших месяцев. У срочной в день планового окончания и
 * в «Ожидает действия» содержимого нет — карточка прячется целиком:
 * пустого контейнера макеты не рисуют (F1, решение владельца 23.09). */
export function hasProgressCard(
  nextPayment: RentalNextPayment | null,
  progress: RentalProgress,
): boolean {
  return (
    nextPayment !== null || progress.totalMonths === null || (progress.monthsRemaining ?? 0) > 0
  );
}

/** Строка бессрочной аренды (решение владельца 2026-09-07): полных месяцев
 * с начала — «Прошло 12 месяцев», глагол как у остатка («Прошёл 21 месяц»);
 * пока не прошёл полный месяц — «Идёт 1 месяц». */
export function rentalElapsedLine(startDate: IsoDate, today: IsoDate): string {
  const elapsed = fullMonthsBetween(startDate, today);
  if (elapsed === 0) {
    return 'Идёт 1 месяц';
  }
  const singular = elapsed % 10 === 1 && elapsed % 100 !== 11;
  return `${singular ? 'Прошёл' : 'Прошло'} ${elapsed} ${monthsWord(elapsed)}`;
}

/** Плата условиями аренды: «56 000 ₽ в месяц» (Figma 1232:61525). */
export function rentAmountPerMonth(amountKopecks: number): string {
  return `${formatMoneyKopecks(amountKopecks)} в месяц`;
}

/** Деньги условий: сумма; не задана — «0 ₽» (макет 1550:94419: залог и
 * комиссия без значения показываются нулём, «Не указано» — только строки). */
function optionalMoney(kopecks: number | null): string {
  return kopecks === null ? formatMoneyKopecks(0) : formatMoneyKopecks(kopecks);
}

/** Три строки карточки «Условия аренды» на детализации (Figma 1232:61522):
 * плата, день оплаты, начало — без года (макет 1232:61524). */
export function rentalTeaserRows(rental: Rental): ReadonlyArray<RentalTermsRow> {
  return [
    { label: 'Арендная плата', value: rentAmountPerMonth(rental.rentPayment.amountKopecks) },
    { label: 'День оплаты', value: paymentDayLabel(rental.rentPayment.paymentDay) },
    { label: 'Начало аренды', value: formatDayMonth(rental.startDate) },
  ];
}

/** Полный экран «Условия аренды» (Figma 1302:53783): восемь строк, пустые
 * значения — «Не указано» (1550:94419); сроки — с годом вне текущего. */
export function rentalTermsRows(rental: Rental, today: IsoDate): ReadonlyArray<RentalTermsRow> {
  const { progress } = rental;
  return [
    { label: 'Арендная плата', value: rentAmountPerMonth(rental.rentPayment.amountKopecks) },
    { label: 'Залог', value: optionalMoney(rental.depositKopecks) },
    { label: 'Комиссия', value: optionalMoney(rental.commissionKopecks) },
    { label: 'День оплаты', value: paymentDayLabel(rental.rentPayment.paymentDay) },
    {
      label: 'Срок аренды',
      value: progress.totalMonths === null
        ? 'Не указано'
        : `${progress.totalMonths} ${monthsWord(progress.totalMonths)}`,
    },
    { label: 'Коммунальные платежи', value: utilitiesLabel(rental.utilities) },
    { label: 'Начало аренды', value: formatDayMonthWithYear(rental.startDate, today) },
    {
      label: 'Окончание аренды',
      value: rental.plannedEndDate === null
        ? 'Не указано'
        : formatDayMonthWithYear(rental.plannedEndDate, today),
    },
  ];
}

/** Текст комментария условий; пустой — «Не указано» (1550:94419). */
export function rentalCommentText(comment: string): string {
  const trimmed = comment.trim();
  return trimmed.length > 0 ? comment : 'Не указано';
}

/** Имя арендатора в строке секции (решение владельца 2026-09-07: без
 * арендатора секция «Арендатор» не выводится вовсе — прежнее «Контакта
 * нет» из #528 переиграно). */
export function rentalTenantTitle(tenant: RentalTenant): string {
  return [tenant.firstName, tenant.lastName].filter((part) => part.length > 0).join(' ');
}

/** Текущая (незавершённая) аренда списка: сервер кладёт её первой
 * (ADR 0053 §4); завершённые — материал «Прошлых аренд» (#535). */
export function currentRentalOf(items: ReadonlyArray<Rental>): Rental | undefined {
  return items.find((rental) => rental.status !== 'completed');
}
