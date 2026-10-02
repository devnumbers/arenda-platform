import type { OperationCreateCommand, PaymentType } from '@/entities/payment';

/**
 * Чистая логика визарда создания одиночной операции (#570): готовность
 * шагов и сериализация в команду POST создания (#569). Поток: сумма →
 * название → категория → (объект — только глобальный вход) → успех. Дата
 * и статус рождается серверными (paid, сегодня владельца) — в команде их
 * нет. Состояние шагов — обычный useState потока (карта #1052, Q2=В:
 * черновика у операции нет — уход со страницы, перезагрузка и закрытие
 * дают чистый лист; он остался только у платежа). Шаги UI — в слое экрана.
 */

/** Форма состояния шагов визарда; живёт, пока смонтирован поток. */
export type OperationWizardDraft = {
  /** Доход/расход — сегмент шага суммы; до явного выбора действует
   * пресет точки входа. */
  readonly type?: PaymentType;
  /** Название операции (шаг 2). */
  readonly title?: string;
  /** Слаг дефолтного каталога (шаг 3). */
  readonly categorySlug?: string;
  /** Сумма в копейках, целая положительная (шаг 1). */
  readonly amountKopecks?: number;
  /** Объект шага «Выбрать объект» (глобальный вход, шаг 4). */
  readonly propertyId?: string;
};

/** Шаги визарда: 4 существует только у глобального входа («Выбрать объект»). */
export type OperationWizardStep = 1 | 2 | 3 | 4;

/** Точка входа: с объекта (объект известен из маршрута) или глобальная. */
export type OperationWizardMode = 'global' | 'property';

/** Пресет направления от точки входа (маршрутный ?type=): Расходы→Расход,
 * Доходы→Доход, иначе (нет/нераспознано) — Расход. */
export function operationPresetFromQueryParam(value: unknown): PaymentType {
  return value === 'income' ? 'income' : 'expense';
}

/** Направление на сегменте шага суммы: явный выбор пользователя или
 * пресет точки входа (Расходы→Расход, Доходы→Доход, иначе Расход). */
export function effectiveOperationType(
  type: PaymentType | undefined,
  preset: PaymentType,
): PaymentType {
  return type ?? preset;
}

/** Готовность шага к продолжению (обязательные поля заполнены). */
export function operationWizardStepReady(
  step: OperationWizardStep,
  draft: OperationWizardDraft,
): boolean {
  switch (step) {
    case 1:
      return draft.amountKopecks !== undefined && draft.amountKopecks > 0;
    case 2:
      // Название необязательно, дефолт — лейбл категории.
      return true;
    case 3:
      return draft.categorySlug !== undefined;
    case 4:
      return draft.propertyId !== undefined;
  }
}

type OperationCreateCommandOptions = {
  /** Объект создания: выбор шага 4 (глобальный вход) или маршрут (с объекта). */
  readonly propertyId: string;
  /** Пресет направления точки входа — действует до явного выбора. */
  readonly presetType: PaymentType;
  /**
   * Дефолтное название из лейбла категории, когда пользователь оставил поле
   * пустым; резолвер передаёт слой каталога — модель визарда от него свободна.
   */
  readonly resolveTitle?: (categorySlug: string) => string | undefined;
};

/** Команда создания из заполненного состояния шагов; undefined — состояние неполно. */
export function buildOperationCreateCommand(
  draft: OperationWizardDraft,
  options: OperationCreateCommandOptions,
): OperationCreateCommand | undefined {
  if (draft.amountKopecks === undefined || draft.amountKopecks <= 0) return undefined;
  if (draft.categorySlug === undefined) return undefined;

  const title =
    draft.title !== undefined && draft.title.trim().length > 0
      ? draft.title.trim()
      : options.resolveTitle?.(draft.categorySlug);
  if (title === undefined) return undefined;

  return {
    type: effectiveOperationType(draft.type, options.presetType),
    title,
    amountKopecks: draft.amountKopecks,
    categorySlug: draft.categorySlug,
  };
}
