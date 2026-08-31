import { describe, expect, it } from 'vitest';
import type { PaymentWizardDraft } from './use-payment-wizard-draft';
import {
  PERIODICITY_OPTIONS,
  WIZARD_TOTAL_STEPS,
  branchKind,
  buildPaymentCreateCommand,
  effectivePaymentForm,
  effectivePaymentType,
  periodicityReady,
  togglePaymentForm,
  togglePaymentType,
  toggleWeekday,
  wizardStepReady,
} from './wizard-model';

const draft = (over: Partial<PaymentWizardDraft>): PaymentWizardDraft => ({
  categorySlug: 'rent',
  title: 'Арендная плата',
  recurrence: { kind: 'monthly', daysOfMonth: [1], lastDay: false },
  endDate: undefined,
  amountKopecks: 250000,
  paymentForm: 'transfer',
  type: 'expense',
  ...over,
});

describe('константы визарда', () => {
  it('пять шагов, меню периодичности без «Один раз»', () => {
    expect(WIZARD_TOTAL_STEPS).toBe(5);
    expect(PERIODICITY_OPTIONS.map((option) => option.label)).toStrictEqual([
      'Каждый день',
      'Каждую неделю',
      'Каждый месяц',
      'Каждый год',
    ]);
  });
});

describe('wizardStepReady — валидация шагов не пускает дальше без обязательного', () => {
  it.each([
    [1, draft({}), true],
    [2, draft({ title: '' }), true], // название необязательно
    [3, draft({ recurrence: { kind: 'daily' } }), true],
    [
      3,
      draft({ recurrence: { kind: 'weekly', weekdays: [] } }),
      false, // ветка недели пуста
    ],
    [
      3,
      draft({ recurrence: { kind: 'weekly', weekdays: [1] } }),
      true,
    ],
    [4, draft({ endDate: undefined }), true], // окончание необязательно
    [5, draft({ amountKopecks: 250000 }), true],
    [5, draft({ amountKopecks: 0 }), false],
    [5, draft({ paymentForm: undefined }), true], // у формы дефолт «перевод»
    [5, draft({ type: undefined }), true], // у типа дефолт «доход»
    [5, draft({ amountKopecks: undefined }), false],
  ] as const)('шаг %s → %s', (step, value, expected) => {
    expect(wizardStepReady(step, value)).toBe(expected);
  });

  it('шаги 1 и 5 не готовы на пустом черновике', () => {
    const empty: PaymentWizardDraft = {};
    expect(wizardStepReady(1, empty)).toBe(false);
    expect(wizardStepReady(2, empty)).toBe(true);
    expect(wizardStepReady(3, empty)).toBe(false);
    expect(wizardStepReady(5, empty)).toBe(false);
  });
});

describe('дефолты и переключатели шага суммы (Figma 834:19662)', () => {
  it('до явного выбора показываются «доход» и «перевод»', () => {
    expect(effectivePaymentType(undefined)).toBe('income');
    expect(effectivePaymentForm(undefined)).toBe('transfer');
  });

  it('явный выбор сильнее дефолта', () => {
    expect(effectivePaymentType('expense')).toBe('expense');
    expect(effectivePaymentForm('cash')).toBe('cash');
  });

  it('клик по чипу меняет значение на альтернативное', () => {
    expect(togglePaymentType('income')).toBe('expense');
    expect(togglePaymentType('expense')).toBe('income');
    expect(togglePaymentForm('transfer')).toBe('cash');
    expect(togglePaymentForm('cash')).toBe('transfer');
  });
});

describe('periodicityReady / branchKind', () => {
  it('ежедневной ветки дат нет; остальные ведут в подбор даты', () => {
    expect(branchKind(undefined)).toBeNull();
    expect(branchKind({ kind: 'daily' })).toBeNull();
    expect(branchKind({ kind: 'weekly', weekdays: [0] })).toBe('weekdays');
    expect(branchKind({ kind: 'monthly', daysOfMonth: [15], lastDay: false })).toBe('monthDays');
    expect(branchKind({ kind: 'yearly', month: 5, day: 13 })).toBe('yearly');
  });

  it('ветки считаются незавершёнными без выбора', () => {
    expect(periodicityReady(undefined)).toBe(false);
    expect(periodicityReady({ kind: 'daily' })).toBe(true);
    expect(periodicityReady({ kind: 'weekly', weekdays: [] })).toBe(false);
    expect(periodicityReady({ kind: 'weekly', weekdays: [6] })).toBe(true);
    expect(periodicityReady({ kind: 'monthly', daysOfMonth: [], lastDay: true })).toBe(true);
    expect(periodicityReady({ kind: 'yearly', month: 12, day: 31 })).toBe(true);
  });
});

describe('toggleWeekday — мультивыбор дней недели', () => {
  it('добавляет выбранный день с сортировкой', () => {
    expect(toggleWeekday([1], 4)).toStrictEqual([1, 4]);
    expect(toggleWeekday([4], 0)).toStrictEqual([0, 4]);
  });

  it('снимает уже выбранный день', () => {
    expect(toggleWeekday([0, 3, 6], 3)).toStrictEqual([0, 6]);
  });

  it('не допускает дубликатов', () => {
    expect(toggleWeekday([], 2)).toStrictEqual([2]);
    expect(toggleWeekday([2, 2], 2)).toStrictEqual([]);
  });
});

describe('buildPaymentCreateCommand — сериализация черновика в команду', () => {
  it('команда совпадает с контрактом POST создания', () => {
    expect(
      buildPaymentCreateCommand(draft({}), { autoPay: false }),
    ).toStrictEqual({
      type: 'expense',
      title: 'Арендная плата',
      amountKopecks: 250000,
      recurrence: { kind: 'monthly', daysOfMonth: [1], lastDay: false },
      paymentForm: 'transfer',
      categorySlug: 'rent',
      autoPay: false,
    });
  });

  it('автоплатёж получает флаг autoPay=true', () => {
    const command = buildPaymentCreateCommand(draft({}), { autoPay: true });
    expect(command?.autoPay).toBe(true);
  });

  it('без явных признаков шага суммы команда берёт дефолты «доход/перевод»', () => {
    const command = buildPaymentCreateCommand(
      draft({ type: undefined, paymentForm: undefined }),
      {},
    );
    expect(command?.type).toBe('income');
    expect(command?.paymentForm).toBe('transfer');
  });

  it('пустое название замещается лейблом категории', () => {
    const command = buildPaymentCreateCommand(
      draft({ title: '' }),
      { autoPay: false, resolveTitle: (slug) => `лейбл:${slug}` },
    );
    expect(command?.title).toBe('лейбл:rent');
  });

  it('без названия и категории команда не строится', () => {
    expect(buildPaymentCreateCommand(draft({ categorySlug: undefined }), {})).toBeUndefined();
    expect(
      buildPaymentCreateCommand(draft({ title: '' }), {
        resolveTitle: () => undefined,
      }),
    ).toBeUndefined();
  });

  it('незавершённый черновик не превращается в команду', () => {
    expect(buildPaymentCreateCommand(draft({ amountKopecks: undefined }), {})).toBeUndefined();
    expect(buildPaymentCreateCommand(draft({ recurrence: { kind: 'weekly', weekdays: [] } }), {})).toBeUndefined();
  });

  it('endDate включается, только когда задан', () => {
    const withEnd = buildPaymentCreateCommand(draft({ endDate: '2027-08-01' }), {});
    expect(withEnd?.endDate).toBe('2027-08-01');
    const openEnded = buildPaymentCreateCommand(draft({}), {});
    expect(openEnded !== undefined && Object.hasOwn(openEnded, 'endDate')).toBe(false);
  });
});
