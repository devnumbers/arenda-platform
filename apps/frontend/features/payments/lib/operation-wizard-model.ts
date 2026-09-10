import type { OperationCreateCommand, PaymentType } from '@/entities/payment';
import type { OperationWizardDraft } from './use-operation-wizard-draft';

/**
 * Чистая логика визарда создания одиночной операции (#570): готовность
 * шагов, восстановление черновика на первый незавершённый шаг и
 * сериализация в команду POST создания (#569). Поток: сумма → название →
 * категория → (объект — только глобальный вход) → успех. Дата и статус
 * рождается серверными (paid, сегодня владельца) — в команде их нет.
 * Шаги UI и хранение черновика — в слое экрана.
 */

/** Шаги визарда: 4 существует только у глобального входа («Выбрать объект»). */
export type OperationWizardStep = 1 | 2 | 3 | 4;

/** Точка входа: с объекта (объект известен из маршрута) или глобальная. */
export type OperationWizardMode = 'global' | 'property';

/** Последний шаг потока: на нём живёт сабмит «Добавить операцию». */
function lastStep(mode: OperationWizardMode): OperationWizardStep {
  return mode === 'global' ? 4 : 3;
}

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

/** Восстановление черновика: первый незавершённый шаг, иначе последний
 * шаг потока — на нём живёт сабмит «Добавить операцию». */
export function initialOperationWizardStep(
  draft: OperationWizardDraft,
  mode: OperationWizardMode,
): OperationWizardStep {
  const last = lastStep(mode);
  for (let step = 1 as OperationWizardStep; step < last; step = (step + 1) as OperationWizardStep) {
    if (!operationWizardStepReady(step, draft)) return step;
  }
  return last;
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

/** Команда создания из завершённого черновика; undefined — черновик неполон. */
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
