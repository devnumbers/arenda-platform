import type {
  IsoDate,
  PaymentCreateCommand,
  PaymentForm,
  PaymentType,
  Recurrence,
} from '@/entities/payment';
import { dateInMonth, isoYear } from '@/shared/lib/calendar';
import type { PaymentWizardDraft } from './use-payment-wizard-draft';

/**
 * Чистая логика визарда создания платежа (#464): готовность шагов,
 * ветки периодичности (без «Один раз» — решение #449), мультивыбор
 * дней недели и сериализация черновика в команду POST создания.
 * Шаги UI и хранение черновика — в слое экрана; здесь только правила.
 */

export const WIZARD_TOTAL_STEPS = 5;

export type WizardStep = 1 | 2 | 3 | 4 | 5;

/** Пункты меню периодичности (Figma 823:11219, «Один раз» убран). */
export const PERIODICITY_OPTIONS = [
  { kind: 'daily', label: 'Каждый день' },
  { kind: 'weekly', label: 'Каждую неделю' },
  { kind: 'monthly', label: 'Каждый месяц' },
  { kind: 'yearly', label: 'Каждый год' },
] as const;

export type PeriodicityKind = (typeof PERIODICITY_OPTIONS)[number]['kind'];

/** Ветка выбора дат; у ежедневного правила её нет. */
export type PeriodicityBranch = 'weekdays' | 'monthDays' | 'yearly';

/** Дни недели метками грида недели (Пн..Вс); значения 0=воскресенье..6=суббота. */
export const WEEKDAY_BUTTONS = [
  { value: 1, label: 'Пн' },
  { value: 2, label: 'Вт' },
  { value: 3, label: 'Ср' },
  { value: 4, label: 'Чт' },
  { value: 5, label: 'Пт' },
  { value: 6, label: 'Сб' },
  { value: 0, label: 'Вс' },
] as const;

/**
 * Ветка периодичности по текущей регулярности черновика. null — ветки нет:
 * «Ежедневно» создаётся сразу с даты заведения (якорь не нужен).
 */
export function branchKind(recurrence: Recurrence | undefined): PeriodicityBranch | null {
  if (recurrence === undefined) return null;
  switch (recurrence.kind) {
    case 'daily':
      return null;
    case 'weekly':
      return 'weekdays';
    case 'monthly':
      return 'monthDays';
    case 'yearly':
      return 'yearly';
  }
}

/** Ближайшее будущее вхождение годового правила — предвыбор бесконечного
 * календаря в ветке «Каждый год»: кандидат в текущем году (несуществующий
 * день прижимается к концу месяца, как на сервере), в прошлом — тот же
 * день следующего года. Год в правиле не хранится (yearly = месяц и день). */
export function yearlyAnchorDate(
  recurrence: { readonly month: number; readonly day: number },
  today: IsoDate,
): IsoDate {
  const month0 = recurrence.month - 1;
  const candidate = dateInMonth(isoYear(today), month0, recurrence.day);
  return candidate < today
    ? dateInMonth(isoYear(today) + 1, month0, recurrence.day)
    : candidate;
}

/** Дефолт ветки при смене вида на weekly/monthly: неполное правило — шаг
 * готов только после выбора дат (по фрейму 1056:53076 ничего не
 * предвыбрано). Годовая ветка сюда не доходит: её правило пишется только
 * подтверждением календаря. */
function defaultForKind(kind: 'weekly' | 'monthly'): Recurrence {
  switch (kind) {
    case 'weekly':
      return { kind: 'weekly', weekdays: [] };
    case 'monthly':
      return { kind: 'monthly', daysOfMonth: [], lastDay: false };
  }
}

/** Итог выбора пункта меню периодичности: что пишется в черновик и какая
 * ветка дат открывается. */
export type PeriodicityPick = {
  /** Новая периодичность черновика; undefined — прежняя готовая сброшена. */
  readonly recurrence: Recurrence | undefined;
  /** Ветка дат для открытия; null — у ежедневного правила её нет. */
  readonly branch: PeriodicityBranch | null;
};

/** Выбор пункта меню (шаг 3): чужая готовая периодичность сбрасывается —
 * у weekly/monthly черновиком становится пустая ветка, у yearly (дефект А
 * #948) правило пишется только подтверждением календаря, поэтому до него
 * готовой периодичности в черновике нет и «Продолжить» старый вид не
 * проведёт. Повторный выбор своего вида хранит готовую ветку. */
export function pickPeriodicityKind(
  kind: PeriodicityKind,
  recurrence: Recurrence | undefined,
): PeriodicityPick {
  if (kind === 'daily') {
    return { recurrence: { kind: 'daily' }, branch: null };
  }
  if (kind === 'yearly') {
    return {
      recurrence: recurrence?.kind === 'yearly' ? recurrence : undefined,
      branch: 'yearly',
    };
  }
  const kept = recurrence?.kind === kind ? recurrence : defaultForKind(kind);
  return { recurrence: kept, branch: branchKind(kept) };
}

/** Регулярность считается выбранной, когда её ветка дат завершена. */
export function periodicityReady(recurrence: Recurrence | undefined): boolean {
  if (recurrence === undefined) return false;
  switch (recurrence.kind) {
    case 'daily':
      return true;
    case 'weekly':
      return recurrence.weekdays.length > 0;
    case 'monthly':
      return recurrence.daysOfMonth.length > 0 || recurrence.lastDay;
    case 'yearly':
      return (
        recurrence.month >= 1
        && recurrence.month <= 12
        && recurrence.day >= 1
        && recurrence.day <= 31
      );
  }
}

/** Готовность шага к продолжению (обязательные поля заполнены). */
export function wizardStepReady(step: WizardStep, draft: PaymentWizardDraft): boolean {
  switch (step) {
    case 1:
      return draft.categorySlug !== undefined;
    case 2:
      // Название необязательно, дефолт — лейбл категории.
      return true;
    case 3:
      return periodicityReady(draft.recurrence);
    case 4:
      // Окончание платежа необязательно: пусто — бессрочный.
      return true;
    case 5:
      // Признаки шага суммы имеют дефолты (Figma 834:19662: чипы сразу
      // показывают «Доход» и «Перевод») — решает только положительная сумма.
      return draft.amountKopecks !== undefined && draft.amountKopecks > 0;
  }
}

/** Дефолтные признаки шага суммы (Figma 834:19662): до явного выбора
 * чипы показывают «Доход» и «Перевод». Живут на слое отображения и
 * сборки команды — черновик хранит только явный выбор пользователя,
 * чтобы «есть ли что продолжать» не зависело от дефолтов. */
const AMOUNT_STEP_DEFAULTS = {
  type: 'income',
  paymentForm: 'transfer',
} as const;

/** Признак типа, видимый на шаге суммы: явный выбор или дефолт. */
export function effectivePaymentType(type: PaymentType | undefined): PaymentType {
  return type ?? AMOUNT_STEP_DEFAULTS.type;
}

/** Признак формы оплаты, видимый на шаге суммы: явный выбор или дефолт. */
export function effectivePaymentForm(paymentForm: PaymentForm | undefined): PaymentForm {
  return paymentForm ?? AMOUNT_STEP_DEFAULTS.paymentForm;
}

/** Клик по чипу-переключателю меняет значение на альтернативное. */
export function togglePaymentType(type: PaymentType): PaymentType {
  return type === 'income' ? 'expense' : 'income';
}

/** Клик по чипу формы оплаты меняет её на альтернативную. */
export function togglePaymentForm(paymentForm: PaymentForm): PaymentForm {
  return paymentForm === 'transfer' ? 'cash' : 'transfer';
}

/** Мультивыбор дней недели: сортированный набор без дубликатов. */
export function toggleWeekday(
  weekdays: ReadonlyArray<number>,
  weekday: number,
): ReadonlyArray<number> {
  if (weekdays.includes(weekday)) {
    return weekdays.filter((day) => day !== weekday);
  }
  return [...weekdays, weekday].sort((a, b) => a - b);
}

export type CreateCommandOptions = {
  /** Тип из шита выбора: автоплатёж создаётся с флагом autoPay. */
  readonly autoPay?: boolean;
  /**
   * Дефолтное название из лейбла категории, когда пользователь оставил поле
   * пустым; резолвер передаёт слой каталога — модель визарда от него свободна.
   */
  readonly resolveTitle?: (categorySlug: string) => string | undefined;
};

/** Команда создания из завершённого черновика; undefined — черновик неполон. */
export function buildPaymentCreateCommand(
  draft: PaymentWizardDraft,
  options: CreateCommandOptions = {},
): PaymentCreateCommand | undefined {
  const recurrence = draft.recurrence;
  if (recurrence === undefined || !periodicityReady(recurrence)) return undefined;
  if (draft.amountKopecks === undefined || draft.amountKopecks <= 0) return undefined;
  if (draft.categorySlug === undefined) return undefined;

  const title =
    draft.title !== undefined && draft.title.trim().length > 0
      ? draft.title.trim()
      : options.resolveTitle?.(draft.categorySlug);
  if (title === undefined) return undefined;

  return {
    type: effectivePaymentType(draft.type),
    title,
    amountKopecks: draft.amountKopecks,
    recurrence,
    paymentForm: effectivePaymentForm(draft.paymentForm),
    categorySlug: draft.categorySlug,
    autoPay: options.autoPay ?? false,
    ...(draft.endDate !== undefined && { endDate: draft.endDate }),
    // Напоминание — только явный выбор (ручная ветка шага 4, карта #822):
    // в контракте создания опущенное поле = напоминаний нет.
    ...(draft.reminderOffsetDays !== undefined && {
      reminderOffsetDays: draft.reminderOffsetDays,
    }),
  };
}
