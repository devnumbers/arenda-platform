import { describe, expect, it } from 'vitest';
import {
  buildOperationCreateCommand,
  effectiveOperationType,
  operationPresetFromQueryParam,
  operationWizardStepReady,
  type OperationWizardDraft,
} from './operation-wizard-model';

describe('operationPresetFromQueryParam', () => {
  it('пресет от точки входа: Доходы→Доход, иначе Расход', () => {
    expect(operationPresetFromQueryParam('income')).toBe('income');
    expect(operationPresetFromQueryParam('expense')).toBe('expense');
    expect(operationPresetFromQueryParam(undefined)).toBe('expense');
    expect(operationPresetFromQueryParam('transfer')).toBe('expense');
    expect(operationPresetFromQueryParam(['income'])).toBe('expense');
  });
});

const FULL_DRAFT: OperationWizardDraft = {
  type: 'expense',
  amountKopecks: 600_000,
  title: 'Аренда мебели',
  categorySlug: 'rent',
  propertyId: 'property-1',
};

describe('effectiveOperationType', () => {
  it('явный выбор пользователя сильнее пресета точки входа', () => {
    expect(effectiveOperationType('income', 'expense')).toBe('income');
    expect(effectiveOperationType('expense', 'income')).toBe('expense');
  });

  it('без явного выбора действует пресет точки входа', () => {
    expect(effectiveOperationType(undefined, 'income')).toBe('income');
    expect(effectiveOperationType(undefined, 'expense')).toBe('expense');
  });
});

describe('operationWizardStepReady', () => {
  it('шаг суммы готов только при положительной сумме', () => {
    expect(operationWizardStepReady(1, {})).toBe(false);
    expect(operationWizardStepReady(1, { amountKopecks: 0 })).toBe(false);
    expect(operationWizardStepReady(1, { amountKopecks: 1 })).toBe(true);
  });

  it('шаг названия необязателен и готов всегда', () => {
    expect(operationWizardStepReady(2, {})).toBe(true);
  });

  it('шаг категории готов после выбора', () => {
    expect(operationWizardStepReady(3, {})).toBe(false);
    expect(operationWizardStepReady(3, { categorySlug: 'rent' })).toBe(true);
  });

  it('шаг объекта готов после выбора объекта', () => {
    expect(operationWizardStepReady(4, {})).toBe(false);
    expect(operationWizardStepReady(4, { propertyId: 'p1' })).toBe(true);
  });
});

describe('buildOperationCreateCommand', () => {
  const options = {
    propertyId: 'property-1',
    presetType: 'expense',
    resolveTitle: (slug: string) => (slug === 'rent' ? 'Аренда' : undefined),
  } as const;

  it('полный черновик даёт команду 1:1 контракту POST', () => {
    expect(
      buildOperationCreateCommand(FULL_DRAFT, options),
    ).toStrictEqual({
      type: 'expense',
      title: 'Аренда мебели',
      amountKopecks: 600_000,
      categorySlug: 'rent',
    });
  });

  it('направление без явного выбора берётся из пресета точки входа', () => {
    const { type: _typed, ...draft } = FULL_DRAFT;
    expect(buildOperationCreateCommand(draft, options)?.type).toBe('expense');
    expect(
      buildOperationCreateCommand(draft, { ...options, presetType: 'income' })?.type,
    ).toBe('income');
  });

  it('пустое название замещается лейблом категории (канон платежей)', () => {
    expect(
      buildOperationCreateCommand(
        { amountKopecks: 100, categorySlug: 'rent', title: '  ' },
        options,
      )?.title,
    ).toBe('Аренда');
  });

  it('название обрезается по краям', () => {
    expect(
      buildOperationCreateCommand(
        { amountKopecks: 100, categorySlug: 'rent', title: ' Мой расход ' },
        options,
      )?.title,
    ).toBe('Мой расход');
  });

  it('объект берётся из опций (вход с объекта), не из черновика', () => {
    const command = buildOperationCreateCommand(
      { ...FULL_DRAFT, propertyId: 'stale-property' },
      { ...options, propertyId: 'route-property' },
    );
    expect(command).toStrictEqual({
      type: 'expense',
      title: 'Аренда мебели',
      amountKopecks: 600_000,
      categorySlug: 'rent',
    });
  });

  it('без суммы, категории или лейбла команда не собирается', () => {
    expect(
      buildOperationCreateCommand(
        { categorySlug: 'rent' },
        options,
      ),
    ).toBeUndefined();
    expect(
      buildOperationCreateCommand({ amountKopecks: 100 }, options),
    ).toBeUndefined();
    expect(
      buildOperationCreateCommand(
        { amountKopecks: 100, categorySlug: 'unknown-slug' },
        options,
      ),
    ).toBeUndefined();
  });
});
