import type { Recurrence } from '@/entities/payment';
import type { PaymentCreateCommand } from '@/entities/payment';
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

/** Регулярность считается выбранной, когда её ветка дат завершена. */
export function periodicityReady(recurrence: Recurrence | undefined): boolean {
  if (recurrence === undefined) return false;
  switch (recurrence.kind) {
    case 'daily':
      return true;
    case 'weekly':
      return recurrence.weekdays.length > 0;
    case 'monthly':
      return recurrence.dayOfMonth >= 1 && recurrence.dayOfMonth <= 31;
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
      return (
        draft.amountKopecks !== undefined
        && draft.amountKopecks > 0
        && draft.type !== undefined
        && draft.paymentForm !== undefined
      );
  }
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
  if (draft.type === undefined || draft.paymentForm === undefined) return undefined;
  if (draft.categorySlug === undefined) return undefined;

  const title =
    draft.title !== undefined && draft.title.trim().length > 0
      ? draft.title.trim()
      : options.resolveTitle?.(draft.categorySlug);
  if (title === undefined) return undefined;

  return {
    type: draft.type,
    title,
    amountKopecks: draft.amountKopecks,
    recurrence,
    paymentForm: draft.paymentForm,
    categorySlug: draft.categorySlug,
    autoPay: options.autoPay ?? false,
    ...(draft.endDate !== undefined && { endDate: draft.endDate }),
  };
}
