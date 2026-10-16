import { addDays, cmp, dateInMonth, type IsoDate, isoMonthNumber, isoYear } from '@/shared/lib/calendar';
import type { PaymentReminderOffset } from '@/entities/payment';
import type {
  RentalCreateCommand,
  RentalPaymentDay,
  RentalUtilities,
} from '@/entities/rental';

/**
 * Чистая логика визарда создания аренды (#530, Figma 1270:46904/37343/46821/
 * 46738): готовность шагов, валидация дат, метки дня оплаты и коммуналки,
 * сериализация черновика в команду POST создания. Шаги UI и хранение
 * черновика — в слое экрана; здесь только правила.
 */

/** Черновик шагов визарда (носитель сессии — rental-wizard-session, карта
 * #1052 D3): покрывает поля всех четырёх шагов; готовность шагов — здесь,
 * шаги UI — слой экрана. */
export type RentalWizardDraft = {
  /** Арендная плата в копейках, целая положительная (шаг 1). */
  readonly amountKopecks?: number;
  /** День оплаты: число месяца 1–31 или «последний день» (шаг 1). */
  readonly paymentDay?: RentalPaymentDay;
  /** Автоплатёж Платежа арендной платы (шаг 3; отсутствие = выключен). */
  readonly autoPay?: boolean;
  /** Лид-тайм напоминания о платеже (шаг 3, карта #822; #1198: отсутствие —
   * «Не напоминать», дефолт шага; в команду уходит явный null). */
  readonly reminderOffsetDays?: PaymentReminderOffset;
  /** Начало аренды — сегодня или позже (шаг 2). */
  readonly startDate?: IsoDate;
  /** Плановое окончание; отсутствие — бессрочная аренда (шаг 2). */
  readonly plannedEndDate?: IsoDate;
  /** Коммунальные платежи (шаг 2). */
  readonly utilities?: RentalUtilities;
  /** Залог в копейках (шаг 2; отсутствие — не задан). */
  readonly depositKopecks?: number;
  /** Комиссия в копейках (шаг 2; отсутствие — не задана). */
  readonly commissionKopecks?: number;
  /** Арендатор — контакт из книги объекта (шаг 4; отсутствие — не выбран). */
  readonly contactId?: string;
};

export const WIZARD_TOTAL_STEPS = 4;

export type RentalWizardStep = 1 | 2 | 3 | 4;

/** Верхняя граница сумм контракта (копейки): 1…10⁹ для платы, 0…10⁹ для
 * залога и комиссии. Ввод сверху срезает маска суммы (9 999 999,99 ₽). */
export const RENTAL_AMOUNT_MAX_KOPECKS = 1_000_000_000;

/** Режимы коммунальных платежей в порядке меню (Figma 1296:48965). */
export const UTILITIES_OPTIONS = [
  { value: 'included', label: 'Включены в стоимость' },
  { value: 'meters_only', label: 'Только счетчики' },
  { value: 'full_receipt', label: 'Вся квитанция' },
] as const satisfies ReadonlyArray<{ value: RentalUtilities; label: string }>;

export function utilitiesLabel(utilities: RentalUtilities): string {
  const option = UTILITIES_OPTIONS.find((candidate) => candidate.value === utilities);
  return option?.label ?? '';
}

/** Метка поля «День оплаты» (Figma 1270:47385 — «10 число»). */
export function paymentDayLabel(paymentDay: RentalPaymentDay): string {
  return paymentDay === 'last' ? 'Последний день месяца' : `${paymentDay} число`;
}

/** Фраза экрана успеха (Figma 1371:63753 — «Каждое 10 число месяца …»). */
export function paymentDayPhrase(paymentDay: RentalPaymentDay): string {
  return paymentDay === 'last'
    ? 'Каждый последний день месяца'
    : `Каждое ${paymentDay} число месяца`;
}

/** Выбор пикера дня → день оплаты аренды: число и «последний день»
 * взаимоисключимы, «последний день» выигрывает (Figma 1270:37490). */
export function paymentDayFromPicker(selection: {
  readonly day: number | undefined;
  readonly last: boolean;
}): RentalPaymentDay | undefined {
  if (selection.last) return 'last';
  return selection.day;
}

/** Начало: сегодня или позже — задним числом аренда не создаётся
 * (словарь аренды; сервер отвечает 400, ADR 0053 §4). */
export function rentalStartDateError(
  startDate: IsoDate | undefined,
  today: IsoDate,
): string | undefined {
  if (startDate === undefined) {
    return 'Выберите дату начала';
  }
  return startDate < today ? 'Начало не может быть в прошлом' : undefined;
}

/** Окончание — строго позже начала (плановое окончание может и не быть
 * задано — бессрочная аренда). */
export function rentalPlannedEndDateError(
  plannedEndDate: IsoDate | undefined,
  startDate: IsoDate | undefined,
): string | undefined {
  if (plannedEndDate === undefined || startDate === undefined) {
    return undefined;
  }
  return cmp(plannedEndDate, startDate) > 0
    ? undefined
    : 'Окончание должно быть позже начала';
}

/** Первая дата дня оплаты на или после старта: 1..30 — свой день (прижатый
 * к длине месяца), «последний день» — фактический последний день месяца.
 * Зеркало backend PaymentDay.FirstPaymentDate (#1154): та же арифметика
 * без дрейфа — месяц строится от числа заново, декабрь перекатывается
 * в январь. Нижняя граница «окна графика» (#1150): окончание раньше неё
 * оставило бы аренду без единого платежа. */
export function firstPaymentDate(startDate: IsoDate, paymentDay: RentalPaymentDay): IsoDate {
  const month0 = isoMonthNumber(startDate) - 1;
  const day = paymentDay === 'last' ? 31 : paymentDay;
  const first = dateInMonth(isoYear(startDate), month0, day);
  return cmp(first, startDate) >= 0
    ? first
    : dateInMonth(isoYear(startDate), month0 + 1, day);
}

/** Минимум пикера планового окончания (#1156): строго позже начала и, в
 * дополнение, не раньше первого вхождения дня оплаты — дата до первой
 * оплаты оставила бы аренду без платежей (инвариант «окна графика»,
 * #1150/#1154). Без дня оплаты работает прежняя граница. */
export function plannedEndDateMinDate(
  startDate: IsoDate,
  paymentDay: RentalPaymentDay | undefined,
): IsoDate {
  const dayAfterStart = addDays(startDate, 1);
  if (paymentDay === undefined) {
    return dayAfterStart;
  }
  const first = firstPaymentDate(startDate, paymentDay);
  return cmp(first, dayAfterStart) > 0 ? first : dayAfterStart;
}

/** Окончание стоит в «окне графика»: строго позже начала и не раньше
 * первого вхождения дня оплаты; без начала или дня правила не вычислить —
 * проверяется только известная часть. Один инвариант для сброса в визарде
 * и в правке условий (#1156). */
export function endCoversSchedule(
  plannedEndDate: IsoDate,
  startDate: IsoDate | undefined,
  paymentDay: RentalPaymentDay | undefined,
): boolean {
  if (startDate === undefined || cmp(plannedEndDate, startDate) <= 0) {
    return false;
  }
  return (
    paymentDay === undefined
    || cmp(plannedEndDate, firstPaymentDate(startDate, paymentDay)) >= 0
  );
}

/** Готовность шага к продолжению (обязательные поля заполнены). После
 * обмена шагов (решение владельца 2026-09-05): шаг 2 — условия, шаг 3 —
 * настройки — всегда (тумблер с дефолтом); шаг 4 — всегда (арендатор
 * необязателен). Кнопка продолжения на неготовом шаге не показывается
 * (решение владельца 2026-09-05) — вместо погашенной. */
export function wizardStepReady(
  step: RentalWizardStep,
  draft: RentalWizardDraft,
  today: IsoDate,
): boolean {
  switch (step) {
    case 1:
      return (
        draft.amountKopecks !== undefined
        && draft.amountKopecks > 0
        && draft.paymentDay !== undefined
      );
    case 2:
      return rentalStartDateError(draft.startDate, today) === undefined
        && rentalPlannedEndDateError(draft.plannedEndDate, draft.startDate) === undefined;
    case 3:
      return true;
    case 4:
      return true;
  }
}

/** Черновик после смены начала аренды (решение владельца 2026-09-05):
 * окончание, переставшее быть позже начала, очищается автоматически —
 * погашенные дни в пикере не оставляют невалидного значения в поле.
 * #1156: окончание, переставшее покрывать первое вхождение дня оплаты
 * (черновик шага 1 его уже знает), очищается так же молча. */
export function draftAfterStartChange(
  draft: RentalWizardDraft,
  startDate: IsoDate | undefined,
): RentalWizardDraft {
  const { plannedEndDate, ...rest } = draft;
  const keepEnd =
    plannedEndDate !== undefined
    && (startDate === undefined
      || endCoversSchedule(plannedEndDate, startDate, draft.paymentDay));
  return {
    ...rest,
    ...(startDate !== undefined && { startDate }),
    ...(keepEnd && { plannedEndDate }),
  };
}

/** Черновик после смены дня оплаты (#1156, канон молчаливого сброса):
 * стоящее окончание, оказавшееся раньше первого вхождения нового дня,
 * очищается — дата, которую пикер окончания больше не даёт выбрать,
 * в черновике не хранится (иначе сабмит ловил бы 400 «окно графика»).
 * Без начала правило не вычислить — окончание переоценит смена начала. */
export function draftAfterPaymentDayChange(
  draft: RentalWizardDraft,
  paymentDay: RentalPaymentDay | undefined,
): RentalWizardDraft {
  const { plannedEndDate, ...rest } = draft;
  const keepEnd =
    plannedEndDate === undefined
    || draft.startDate === undefined
    || endCoversSchedule(plannedEndDate, draft.startDate, paymentDay);
  return {
    ...rest,
    ...(paymentDay !== undefined && { paymentDay }),
    ...(keepEnd && { plannedEndDate }),
  };
}

/** Черновик → команда создания; недостроенный или некорректный черновик
 * команды не даёт (кнопка сабмита уже притушена готовностью шагов). */
export function buildRentalCreateCommand(
  draft: RentalWizardDraft,
  today: IsoDate,
): RentalCreateCommand | undefined {
  const { amountKopecks, paymentDay, startDate } = draft;
  if (
    amountKopecks === undefined
    || amountKopecks <= 0
    || amountKopecks > RENTAL_AMOUNT_MAX_KOPECKS
    || paymentDay === undefined
    || startDate === undefined
    || rentalStartDateError(startDate, today) !== undefined
    || rentalPlannedEndDateError(draft.plannedEndDate, startDate) !== undefined
  ) {
    return undefined;
  }

  return {
    amountKopecks,
    paymentDay,
    startDate,
    plannedEndDate: draft.plannedEndDate ?? null,
    utilities: draft.utilities ?? 'included',
    depositKopecks: clampAmount(draft.depositKopecks),
    commissionKopecks: clampAmount(draft.commissionKopecks),
    contactId: draft.contactId ?? null,
    autoPay: draft.autoPay ?? false,
    // Дефолт «Не напоминать» (#1198): выбор «за N дней» протекает 1:1,
    // отсутствие выбора едет явным null — дефолт не протекает в команду
    // молча.
    reminderOffsetDays: draft.reminderOffsetDays ?? null,
  };
}

/** Залог/комиссия за границей контракта (0…10⁹ копеек) командой не идут. */
function clampAmount(kopecks: number | undefined): number | null {
  if (kopecks === undefined || kopecks < 0 || kopecks > RENTAL_AMOUNT_MAX_KOPECKS) {
    return null;
  }
  return kopecks;
}
