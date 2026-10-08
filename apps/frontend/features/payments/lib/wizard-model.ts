import type {
  IsoDate,
  PaymentCreateCommand,
  PaymentType,
  Recurrence,
} from '@/entities/payment';
import { firstOccurrence } from '@/entities/payment';
import { cmp, dateInMonth, isoYear } from '@/shared/lib/calendar';
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
      // У типа есть дефолт («Доход») — решает только положительная сумма.
      return draft.amountKopecks !== undefined && draft.amountKopecks > 0;
  }
}

/** Восстановление черновика без штампа шага: первый незавершённый шаг
 * (опциональные 2 и 4 перепрыгиваются). */
function initialStep(draft: PaymentWizardDraft): WizardStep {
  if (!wizardStepReady(1, draft)) return 1;
  if (!wizardStepReady(3, draft)) return 3;
  return 5;
}

/** Неизвестное значение — номер шага визарда (целое 1..5): диапазонная
 * проверка persisted штампа; гарду верит и валидатор черновика, и resume. */
export function isWizardStep(value: unknown): value is WizardStep {
  return (
    typeof value === 'number'
    && Number.isInteger(value)
    && value >= 1
    && value <= WIZARD_TOTAL_STEPS
  );
}

/** Шаг при возобновлении черновика (#1055): валидный сохранённый штамп
 * перехода сильнее пересчёта из заполненности — так resume возвращает на
 * точный шаг, включая опциональные 2 и 4. Клэмпа «не выше первого
 * незавершённого» нет: канон «шаг-подсказка» (карта #1052, спека #1054). */
export function resumePaymentWizardStep(draft: PaymentWizardDraft): WizardStep {
  return isWizardStep(draft.step) ? draft.step : initialStep(draft);
}

/** Черновик после перехода на шаг next («Назад» — тот же штамп): номер
 * шага едет в persisted payload; полевые правки шаг не трогают (спред
 * в потоке). */
export function wizardDraftAfterStep(
  draft: PaymentWizardDraft,
  next: WizardStep,
): PaymentWizardDraft {
  return { ...draft, step: next };
}

/** Черновик после смены периодичности (#1155, решение владельца
 * 2026-10-06): стоящее окончание, оказавшееся раньше первого вхождения
 * нового расписания, сбрасывается молча — дата, которую пикер больше не
 * даёт выбрать, в черновике не хранится (иначе сабмит ловил бы 400
 * «окно графика»). undefined — прежняя готовая ветка сброшена
 * (дефект А #948): расписание неизвестно, окончание переоценит следующее
 * применение. Расписание строится от «сегодня» владельца: при создании
 * сервер ставит since сам (ADR 0048), пауз у нового правила нет. */
export function draftAfterRecurrenceChange(
  draft: PaymentWizardDraft,
  recurrence: Recurrence | undefined,
  today: IsoDate,
): PaymentWizardDraft {
  if (recurrence === undefined) {
    return { ...draft, recurrence: undefined };
  }
  if (draft.endDate === undefined) {
    return { ...draft, recurrence };
  }
  const first = firstOccurrence({ recurrence, since: today, endDate: undefined, pauses: [] });
  return first !== null && cmp(first, draft.endDate) > 0
    ? { ...draft, recurrence, endDate: undefined }
    : { ...draft, recurrence };
}

/** Дефолт типа шага суммы: до явного выбора чип показывает «Доход»
 * (форма оплаты снесена — карта #1005, #1008). Живёт на слое отображения
 * и сборки команды — черновик хранит только явный выбор пользователя,
 * чтобы «есть ли что продолжать» не зависело от дефолта. */
const AMOUNT_STEP_DEFAULTS = {
  type: 'income',
} as const;

/** Признак типа, видимый на шаге суммы: явный выбор или дефолт. */
export function effectivePaymentType(type: PaymentType | undefined): PaymentType {
  return type ?? AMOUNT_STEP_DEFAULTS.type;
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
    categorySlug: draft.categorySlug,
    autoPay: options.autoPay ?? false,
    ...(draft.endDate !== undefined && { endDate: draft.endDate }),
    // Напоминание — только явный выбор (ручная ветка шага 4, карта #822):
    // в контракте создания опущенное поле = напоминаний нет.
    ...(draft.reminderOffsetDays !== undefined && {
      reminderOffsetDays: draft.reminderOffsetDays,
    }),
    // Уведомление об автоплатеже — только у автоплатежа и только явное
    // «Да, уведомлять» (макет 3214-72739): опущенное поле = «Не
    // уведомлять», сервер ставит false (#1189).
    ...(options.autoPay === true
      && draft.notifyAutoPaid === true && { notifyAutoPaid: true }),
  };
}
