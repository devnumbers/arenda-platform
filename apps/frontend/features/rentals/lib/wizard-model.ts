import { cmp, type IsoDate } from '@/shared/lib/calendar';
import type {
  RentalCreateCommand,
  RentalPaymentDay,
  RentalUtilities,
} from '@/entities/rental';
import type { RentalWizardDraft } from './use-rental-wizard-draft';

/**
 * Чистая логика визарда создания аренды (#530, Figma 1270:46904/37343/46821/
 * 46738): готовность шагов, валидация дат, метки дня оплаты и коммуналки,
 * сериализация черновика в команду POST создания. Шаги UI и хранение
 * черновика — в слое экрана; здесь только правила.
 */

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

/** Готовность шага к продолжению (обязательные поля заполнены). Шаг 2 —
 * всегда (тумблер с дефолтом), шаг 4 — всегда (арендатор необязателен). */
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
      return true;
    case 3:
      return rentalStartDateError(draft.startDate, today) === undefined
        && rentalPlannedEndDateError(draft.plannedEndDate, draft.startDate) === undefined;
    case 4:
      return true;
  }
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
  };
}

/** Залог/комиссия за границей контракта (0…10⁹ копеек) командой не идут. */
function clampAmount(kopecks: number | undefined): number | null {
  if (kopecks === undefined || kopecks < 0 || kopecks > RENTAL_AMOUNT_MAX_KOPECKS) {
    return null;
  }
  return kopecks;
}
