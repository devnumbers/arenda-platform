import { describe, expect, it } from 'vitest';
import type { PaymentWizardDraft } from './use-payment-wizard-draft';
import {
  PERIODICITY_OPTIONS,
  WIZARD_TOTAL_STEPS,
  branchKind,
  buildPaymentCreateCommand,
  effectivePaymentType,
  periodicityReady,
  pickPeriodicityKind,
  resumePaymentWizardStep,
  toggleWeekday,
  draftAfterRecurrenceChange,
  wizardDraftAfterStep,
  wizardStepReady,
  yearlyAnchorDate,
} from './wizard-model';

const draft = (over: Partial<PaymentWizardDraft>): PaymentWizardDraft => ({
  categorySlug: 'rent',
  title: 'Арендная плата',
  recurrence: { kind: 'monthly', daysOfMonth: [1], lastDay: false },
  endDate: undefined,
  amountKopecks: 250000,
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

describe('дефолты и переключатели шага суммы', () => {
  it('до явного выбора показывается «доход»', () => {
    expect(effectivePaymentType(undefined)).toBe('income');
  });

  it('явный выбор сильнее дефолта', () => {
    expect(effectivePaymentType('expense')).toBe('expense');
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

describe('yearlyAnchorDate — ближайшее будущее вхождение годового правила', () => {
  it('месяц впереди — вхождение в текущем году', () => {
    expect(yearlyAnchorDate({ month: 12, day: 31 }, '2026-09-04')).toBe('2026-12-31');
  });

  it('месяц позади — вхождение в следующем году', () => {
    expect(yearlyAnchorDate({ month: 1, day: 15 }, '2026-09-04')).toBe('2027-01-15');
  });

  it('тот же месяц, день раньше сегодняшнего — следующий год', () => {
    expect(yearlyAnchorDate({ month: 9, day: 1 }, '2026-09-04')).toBe('2027-09-01');
  });

  it('тот же день — сегодня и есть ближайшее вхождение', () => {
    expect(yearlyAnchorDate({ month: 9, day: 4 }, '2026-09-04')).toBe('2026-09-04');
  });

  it('несуществующий день прижимается к концу месяца (29.02, 31-е)', () => {
    expect(yearlyAnchorDate({ month: 2, day: 29 }, '2026-09-04')).toBe('2027-02-28');
    expect(yearlyAnchorDate({ month: 4, day: 31 }, '2026-09-04')).toBe('2027-04-30');
  });

  it('прижатый день в високосном году остаётся 29 февраля', () => {
    // Как сервер: вхождение {29.02} в невисокосный год — 28.02, поэтому
    // ближайшее будущее вхождение от 01.03.2026 — 28.02.2027, не високосный
    // 2028-й; пользователь может отскроллить и выбрать 29.02.2028 заново.
    expect(yearlyAnchorDate({ month: 2, day: 29 }, '2026-03-01')).toBe('2027-02-28');
  });
});

describe('draftAfterRecurrenceChange — молчаливый сброс endDate (#1155)', () => {
  const fridays = { kind: 'weekly', weekdays: [5] } as const;

  it('окончание между today и первым вхождением нового правила сбрасывается', () => {
    // weekly «пт», today = ср 07.10: первое вхождение 09.10, окончание 08.10
    // — то самое окно без единого платежа (#1150); в черновике endDate нет.
    const next = draftAfterRecurrenceChange(
      draft({ endDate: '2026-10-08' }),
      fridays,
      '2026-10-07',
    );
    expect(next.recurrence).toStrictEqual(fridays);
    expect(next.endDate).toBeUndefined();
    // Прочие поля шагов на месте.
    expect(next.title).toBe('Арендная плата');
    expect(next.amountKopecks).toBe(250_000);
  });

  it('окончание в день первого вхождения валидно — сохраняется', () => {
    const next = draftAfterRecurrenceChange(
      draft({ endDate: '2026-10-09' }),
      fridays,
      '2026-10-07',
    );
    expect(next.endDate).toBe('2026-10-09');
  });

  it('окончание позже первого вхождения сохраняется', () => {
    const next = draftAfterRecurrenceChange(
      draft({ endDate: '2026-12-31' }),
      fridays,
      '2026-10-07',
    );
    expect(next.endDate).toBe('2026-12-31');
  });

  it('без окончания сбросить нечего — правило просто пишется', () => {
    const next = draftAfterRecurrenceChange(draft({}), fridays, '2026-10-07');
    expect(next.recurrence).toStrictEqual(fridays);
    expect(next.endDate).toBeUndefined();
  });

  it('daily: первое вхождение = today, окончание-сегодня сохраняется', () => {
    const next = draftAfterRecurrenceChange(
      draft({ endDate: '2026-10-07' }),
      { kind: 'daily' },
      '2026-10-07',
    );
    expect(next.endDate).toBe('2026-10-07');
  });

  it('monthly 31-е: первое вхождение прижимается к длине месяца', () => {
    // today = 10.11: первое вхождение 30.11; окончание 28.11 — дыра, сброс.
    const monthly31 = { kind: 'monthly', daysOfMonth: [31], lastDay: false } as const;
    expect(
      draftAfterRecurrenceChange(
        draft({ endDate: '2026-11-28' }),
        monthly31,
        '2026-11-10',
      ).endDate,
    ).toBeUndefined();
    expect(
      draftAfterRecurrenceChange(
        draft({ endDate: '2026-11-30' }),
        monthly31,
        '2026-11-10',
      ).endDate,
    ).toBe('2026-11-30');
  });

  it('yearly: далёкое первое вхождение сбрасывает близкое окончание', () => {
    const next = draftAfterRecurrenceChange(
      draft({ endDate: '2026-12-31' }),
      { kind: 'yearly', month: 2, day: 29 },
      '2026-10-07',
    );
    // Первое вхождение 29.02.2028 — окончание-2026 раньше него.
    expect(next.endDate).toBeUndefined();
  });

  it('сброс ветки года (recurrence undefined) endDate не трогает', () => {
    const next = draftAfterRecurrenceChange(
      draft({ endDate: '2026-10-08', recurrence: fridays }),
      undefined,
      '2026-10-07',
    );
    expect(next.recurrence).toBeUndefined();
    expect(next.endDate).toBe('2026-10-08');
  });
});

describe('pickPeriodicityKind — выбор пункта меню периодичности (#995)', () => {
  const monthlyReady = { kind: 'monthly', daysOfMonth: [15], lastDay: false } as const;

  it('ежедневное правило готово сразу, ветки дат нет', () => {
    expect(pickPeriodicityKind('daily', monthlyReady)).toStrictEqual({
      recurrence: { kind: 'daily' },
      branch: null,
    });
  });

  it('weekly сбрасывает готовый месяц в пустую ветку недели', () => {
    expect(pickPeriodicityKind('weekly', monthlyReady)).toStrictEqual({
      recurrence: { kind: 'weekly', weekdays: [] },
      branch: 'weekdays',
    });
  });

  it('monthly готовит пустую ветку месяца на чужом черновике', () => {
    expect(pickPeriodicityKind('monthly', { kind: 'weekly', weekdays: [1] })).toStrictEqual({
      recurrence: { kind: 'monthly', daysOfMonth: [], lastDay: false },
      branch: 'monthDays',
    });
  });

  it('годовая ветка сбрасывает прежнюю готовую периодичность (дефект А #948): до подтверждения календаря правила нет', () => {
    expect(pickPeriodicityKind('yearly', monthlyReady)).toStrictEqual({
      recurrence: undefined,
      branch: 'yearly',
    });
    expect(pickPeriodicityKind('yearly', { kind: 'daily' })).toStrictEqual({
      recurrence: undefined,
      branch: 'yearly',
    });
  });

  it('повторный выбор годовой хранит подтверждённое правило (календарь предвыбирает якорь)', () => {
    const yearly = { kind: 'yearly', month: 10, day: 15 } as const;
    expect(pickPeriodicityKind('yearly', yearly)).toStrictEqual({
      recurrence: yearly,
      branch: 'yearly',
    });
  });

  it('повторный выбор своего вида хранит готовую ветку (weekly/monthly)', () => {
    const weekly = { kind: 'weekly', weekdays: [2, 5] } as const;
    expect(pickPeriodicityKind('weekly', weekly)).toStrictEqual({
      recurrence: weekly,
      branch: 'weekdays',
    });
    expect(pickPeriodicityKind('monthly', monthlyReady)).toStrictEqual({
      recurrence: monthlyReady,
      branch: 'monthDays',
    });
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
      categorySlug: 'rent',
      autoPay: false,
    });
  });

  it('автоплатёж получает флаг autoPay=true', () => {
    const command = buildPaymentCreateCommand(draft({}), { autoPay: true });
    expect(command?.autoPay).toBe(true);
  });

  it('без явного типа команда берёт дефолт «доход»', () => {
    const command = buildPaymentCreateCommand(draft({ type: undefined }), {});
    expect(command?.type).toBe('income');
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

  it('напоминание включается, только когда выбрано (ручная ветка шага 4, #822)', () => {
    const withReminder = buildPaymentCreateCommand(draft({ reminderOffsetDays: 3 }), {});
    expect(withReminder?.reminderOffsetDays).toBe(3);
    const without = buildPaymentCreateCommand(draft({}), {});
    expect(without !== undefined && Object.hasOwn(without, 'reminderOffsetDays')).toBe(false);
  });

  it('уведомление об автоплатеже едет в команду только у автоплатежа и только явное «да» (#1193)', () => {
    // «Да, уведомлять» у автоплатежа — флаг в контракте.
    const notified = buildPaymentCreateCommand(draft({ notifyAutoPaid: true }), {
      autoPay: true,
    });
    expect(notified?.notifyAutoPaid).toBe(true);
    // «Не уведомлять» (дефолт, макет 3214-72739) — поле опускается,
    // сервер ставит false (#1189).
    const silent = buildPaymentCreateCommand(draft({}), { autoPay: true });
    expect(silent !== undefined && Object.hasOwn(silent, 'notifyAutoPaid')).toBe(false);
    // У обычного платежа радио нет — черновик-мусор не протекает.
    const regular = buildPaymentCreateCommand(draft({ notifyAutoPaid: true }), {
      autoPay: false,
    });
    expect(regular !== undefined && Object.hasOwn(regular, 'notifyAutoPaid')).toBe(false);
  });
});

describe('resumePaymentWizardStep — сохранённый шаг сильнее пересчёта (#1055)', () => {
  it('валидный сохранённый шаг открывается, даже если пересчёт дал бы другой', () => {
    // Шаг 1 и 3 готовы — пересчёт дал бы 5, но пользователь стоял на 2.
    expect(resumePaymentWizardStep(draft({ step: 2 }))).toBe(2);
    // Опциональный шаг 4: пересчёт перепрыгнул бы на 5.
    expect(resumePaymentWizardStep(draft({ step: 4 }))).toBe(4);
  });

  it('нет штампа — пересчёт из заполненности (старые черновики совместимы)', () => {
    expect(resumePaymentWizardStep(draft({ step: undefined }))).toBe(5);
    // Периодичность не выбрана — первый незавершённый это шаг 3.
    expect(resumePaymentWizardStep(draft({ step: undefined, recurrence: undefined }))).toBe(3);
    expect(resumePaymentWizardStep(draft({ step: undefined, categorySlug: undefined, recurrence: undefined }))).toBe(1);
  });

  it('мусорный штамп — пересчёт (валидатор обычно отбрасывает поле, resume подстраховывает)', () => {
    expect(resumePaymentWizardStep(draft({ step: 7 }))).toBe(5);
    expect(resumePaymentWizardStep(draft({ step: 0 }))).toBe(5);
  });

  it('пустой черновик — шаг 1', () => {
    const empty: PaymentWizardDraft = {};
    expect(resumePaymentWizardStep(empty)).toBe(1);
  });
});

describe('wizardDraftAfterStep — штамп перехода пишется в пейлоад (#1055)', () => {
  it('переход ставит step, остальные поля на месте', () => {
    expect(wizardDraftAfterStep(draft({}), 2)).toStrictEqual(draft({ step: 2 }));
  });

  it('каждый переход перезаписывает прежний штамп', () => {
    expect(wizardDraftAfterStep(draft({ step: 4 }), 5)).toStrictEqual(draft({ step: 5 }));
    // «Назад» тоже штампует: возврат на шаг 1 запоминается как шаг 1.
    expect(wizardDraftAfterStep(draft({ step: 2 }), 1)).toStrictEqual(draft({ step: 1 }));
  });
});
