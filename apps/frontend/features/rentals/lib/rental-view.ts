import type { IsoDate } from '@/shared/lib/calendar';
import { daysUntil, fullMonthsBetween, inclusiveDays } from '@/shared/lib/calendar';
import { daysWord } from '@/shared/lib/days-word';
import { formatDayMonthWithYear, formatOverdueDays } from '@/shared/lib/date-format';
import { formatMoneyKopecks, ratioToPercent } from '@/shared/lib/format-money';
import { monthsWord } from '@/shared/lib/months-word';
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

/** Красная строка просрочки карточки прогресса (#817, решение владельца
 * 23.09): «Просрочен 1 месяц» / «Просрочено 2 месяца»; единственное число —
 * у 1, 21, 31… (11, 111 — множительное). N — платёжные месяцы, счётчик
 * серверный (ADR 0053 §2). */
export function rentalOverdueLine(overdueMonths: number): string {
  const verb = pluralize(overdueMonths, 'Просрочен', 'Просрочено', 'Просрочено');
  return `${verb} ${overdueMonths} ${monthsWord(overdueMonths)}`;
}

/** Карточка прогресса на детализации видима, пока ей есть что жить: со
 * следующим платежом — строка дней и бар (1232:61259), без него — строка
 * остатка или прошедших месяцев, просрочка — красная строка (#817). Строка
 * остатка у срочной живёт, пока план не наступил: месяцы, а в последние
 * недели — дни (#1387). В сам день планового окончания содержимого нет —
 * карточка прячется целиком: пустого контейнера макеты не рисуют (F1,
 * решение владельца 23.09; «Заканчивается сегодня» этого дня живёт в футере
 * блока объекта и бейдже); задолженность появляется — содержимое снова
 * есть. */
export function hasProgressCard(rental: Rental): boolean {
  return (
    rental.rentPayment.nextPayment !== null ||
    rental.progress.overdueMonths !== null ||
    rental.progress.totalMonths === null ||
    (rental.plannedEndDate !== null && daysUntil(rental.today, rental.plannedEndDate) > 0)
  );
}

/** Строка бессрочной аренды (решение владельца 2026-09-07): полных месяцев
 * с начала — «Прошло 12 месяцев», глагол как у остатка («Прошёл 21 месяц»);
 * пока не прошёл полный месяц — точные дни включительно («Идёт 3 дня»,
 * «Идёт 1 день»; решение владельца 22.10 — вместо «Идёт 1 месяц»; будущий
 * старт кверк-класс прежнего «Идёт 1 месяц» — прижат к 1 дню). */
export function rentalElapsedLine(startDate: IsoDate, today: IsoDate): string {
  const elapsed = fullMonthsBetween(startDate, today);
  if (elapsed === 0) {
    const days = Math.max(1, inclusiveDays(startDate, today));
    return `Идёт ${days} ${daysWord(days)}`;
  }
  const verb = pluralize(elapsed, 'Прошёл', 'Прошло', 'Прошло');
  return `${verb} ${elapsed} ${monthsWord(elapsed)}`;
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
 * плата, день оплаты, начало — год вне текущего (правило владельца 13.10;
 * макет 1232:61524 без года старше правила — переиграно #1385). */
export function rentalTeaserRows(rental: Rental): ReadonlyArray<RentalTermsRow> {
  return [
    { label: 'Арендная плата', value: rentAmountPerMonth(rental.rentPayment.amountKopecks) },
    { label: 'День оплаты', value: paymentDayLabel(rental.rentPayment.paymentDay) },
    { label: 'Начало аренды', value: formatDayMonthWithYear(rental.startDate, rental.today) },
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
