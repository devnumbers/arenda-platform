import { describe, expect, it } from 'vitest';
import {
  buildRentalCreateCommand,
  draftAfterPaymentDayChange,
  draftAfterStartChange,
  firstPaymentDate,
  paymentDayFromPicker,
  paymentDayLabel,
  paymentDayPhrase,
  plannedEndDateMinDate,
  rentalPlannedEndDateError,
  rentalStartDateError,
  utilitiesLabel,
  wizardStepReady,
} from './wizard-model';
import type { RentalWizardDraft } from './wizard-model';

const TODAY = '2026-09-05';

const FULL_DRAFT: RentalWizardDraft = {
  amountKopecks: 5600000,
  paymentDay: 10,
  autoPay: false,
  reminderOffsetDays: 3,
  startDate: '2026-09-10',
  plannedEndDate: '2027-09-09',
  utilities: 'full_receipt',
  depositKopecks: 3000000,
  commissionKopecks: 500000,
  contactId: '0198c7a2-0000-7000-8000-000000000003',
};

describe('wizardStepReady', () => {
  it('шаг 1 готов, когда есть положительная сумма и день оплаты', () => {
    expect(wizardStepReady(1, {}, TODAY)).toBe(false);
    expect(wizardStepReady(1, { amountKopecks: 5600000 }, TODAY)).toBe(false);
    expect(wizardStepReady(1, { paymentDay: 10 }, TODAY)).toBe(false);
    expect(wizardStepReady(1, { amountKopecks: 0, paymentDay: 10 }, TODAY)).toBe(false);
    expect(wizardStepReady(1, { amountKopecks: 100, paymentDay: 'last' }, TODAY)).toBe(true);
  });

  it('шаг 2 готов, когда начало задано и не в прошлом (условия)', () => {
    expect(wizardStepReady(2, {}, TODAY)).toBe(false);
    expect(wizardStepReady(2, { startDate: '2026-09-04' }, TODAY)).toBe(false);
    expect(wizardStepReady(2, { startDate: TODAY }, TODAY)).toBe(true);
    expect(wizardStepReady(2, { startDate: '2026-10-01' }, TODAY)).toBe(true);
  });

  it('шаг 3 готов всегда — тумблер имеет дефолт (настройки)', () => {
    expect(wizardStepReady(3, {}, TODAY)).toBe(true);
  });

  it('шаг 4 готов всегда — арендатор необязателен', () => {
    expect(wizardStepReady(4, {}, TODAY)).toBe(true);
  });
});

describe('draftAfterStartChange', () => {
  it('начало сохраняется; валидное окончание остаётся', () => {
    expect(draftAfterStartChange({ plannedEndDate: '2027-09-09' }, '2026-09-10')).toStrictEqual({
      startDate: '2026-09-10',
      plannedEndDate: '2027-09-09',
    });
  });

  it('окончание, переставшее быть позже начала, очищается', () => {
    expect(draftAfterStartChange({ plannedEndDate: '2026-09-10' }, '2026-09-10')).toStrictEqual({
      startDate: '2026-09-10',
    });
    expect(draftAfterStartChange({ plannedEndDate: '2026-09-09' }, '2026-09-10')).toStrictEqual({
      startDate: '2026-09-10',
    });
  });

  it('окончание позже начала, но раньше первого вхождения дня оплаты, очищается (#1156)', () => {
    // Research-кейс #1150: старт 10-го, день оплаты 25-е, окончание 20-е
    // — аренда без единого платежа; такое окончание в черновике не живёт.
    expect(
      draftAfterStartChange(
        { paymentDay: 25, plannedEndDate: '2026-10-20' },
        '2026-10-10',
      ),
    ).toStrictEqual({ paymentDay: 25, startDate: '2026-10-10' });
  });

  it('окончание, равное первому вхождению, остаётся (включительная граница)', () => {
    expect(
      draftAfterStartChange(
        { paymentDay: 25, plannedEndDate: '2026-10-25' },
        '2026-10-10',
      ),
    ).toStrictEqual({ paymentDay: 25, startDate: '2026-10-10', plannedEndDate: '2026-10-25' });
  });

  it('день оплаты в тот же день, что начало: работает прежняя граница «строго позже начала»', () => {
    expect(
      draftAfterStartChange(
        { paymentDay: 10, plannedEndDate: '2026-10-11' },
        '2026-10-10',
      ),
    ).toStrictEqual({ paymentDay: 10, startDate: '2026-10-10', plannedEndDate: '2026-10-11' });
  });

  it('прочие поля переносятся', () => {
    expect(
      draftAfterStartChange(
        { amountKopecks: 100, paymentDay: 5, plannedEndDate: '2026-10-10' },
        '2026-09-11',
      ),
    ).toStrictEqual({
      amountKopecks: 100,
      paymentDay: 5,
      startDate: '2026-09-11',
      plannedEndDate: '2026-10-10',
    });
  });

  it('без начала окончание не трогается', () => {
    expect(draftAfterStartChange({ plannedEndDate: '2026-09-10' }, undefined)).toStrictEqual({
      plannedEndDate: '2026-09-10',
    });
  });
});

describe('draftAfterPaymentDayChange', () => {
  it('день оплаты, сдвинувший первое вхождение за стоящее окончание, очищает его (#1156)', () => {
    // Было: день 10-е, начало 10-го, окончание 20-е — валидно. Стало:
    // день 25-е, первое вхождение 25-е позже окончания 20-го.
    expect(
      draftAfterPaymentDayChange(
        { startDate: '2026-10-10', plannedEndDate: '2026-10-20' },
        25,
      ),
    ).toStrictEqual({ startDate: '2026-10-10', paymentDay: 25 });
  });

  it('окончание, покрывающее новое первое вхождение, остаётся', () => {
    expect(
      draftAfterPaymentDayChange(
        { startDate: '2026-10-10', plannedEndDate: '2026-11-20' },
        25,
      ),
    ).toStrictEqual({ startDate: '2026-10-10', plannedEndDate: '2026-11-20', paymentDay: 25 });
  });

  it('«последний день месяца» — первое вхождение в конце месяца', () => {
    expect(
      draftAfterPaymentDayChange(
        { startDate: '2026-10-10', plannedEndDate: '2026-10-20' },
        'last',
      ),
    ).toStrictEqual({ startDate: '2026-10-10', paymentDay: 'last' });
  });

  it('прочие поля переносятся; без начала окончание не трогается', () => {
    expect(
      draftAfterPaymentDayChange(
        { amountKopecks: 100, plannedEndDate: '2026-10-20' },
        25,
      ),
    ).toStrictEqual({ amountKopecks: 100, plannedEndDate: '2026-10-20', paymentDay: 25 });
  });
});

describe('firstPaymentDate', () => {
  it('день позже начала в том же месяце — он и есть первое вхождение (research-кейс #1150)', () => {
    expect(firstPaymentDate('2026-10-10', 25)).toBe('2026-10-25');
  });

  it('день раньше начала уходит в следующий месяц', () => {
    expect(firstPaymentDate('2026-10-10', 5)).toBe('2026-11-05');
  });

  it('декабрь перекатывается в январь следующего года', () => {
    expect(firstPaymentDate('2026-12-10', 5)).toBe('2027-01-05');
  });

  it('«последний день месяца» — фактический конец месяца', () => {
    expect(firstPaymentDate('2026-10-10', 'last')).toBe('2026-10-31');
  });

  it('день, которого в месяце нет, прижимается к его длине (зеркало PaymentDay.FirstPaymentDate)', () => {
    expect(firstPaymentDate('2026-11-10', 31)).toBe('2026-11-30');
    expect(firstPaymentDate('2026-02-10', 30)).toBe('2026-02-28');
    expect(firstPaymentDate('2026-02-10', 'last')).toBe('2026-02-28');
  });

  it('день оплаты в сам день старта — первое вхождение совпадает с началом', () => {
    expect(firstPaymentDate('2026-10-10', 10)).toBe('2026-10-10');
  });
});

describe('plannedEndDateMinDate', () => {
  it('первое вхождение дня оплаты позже дня после начала — ведёт оно (research-кейс #1150)', () => {
    expect(plannedEndDateMinDate('2026-10-10', 25)).toBe('2026-10-25');
  });

  it('первое вхождение в день старта или раньше дня после начала — работает прежняя граница «строго позже начала»', () => {
    expect(plannedEndDateMinDate('2026-10-10', 10)).toBe('2026-10-11');
    expect(plannedEndDateMinDate('2026-10-10', 11)).toBe('2026-10-11');
  });

  it('«последний день месяца» ведёт, когда он позже дня после начала', () => {
    expect(plannedEndDateMinDate('2026-10-10', 'last')).toBe('2026-10-31');
  });

  it('без дня оплаты — прежняя граница «строго позже начала»', () => {
    expect(plannedEndDateMinDate('2026-10-10', undefined)).toBe('2026-10-11');
  });
});

describe('rentalStartDateError', () => {
  it('пустое начало — ошибка обязательного поля', () => {
    expect(rentalStartDateError(undefined, TODAY)).toBe('Выберите дату начала');
  });

  it('начало в прошлом нельзя — задним числом аренда не создаётся', () => {
    expect(rentalStartDateError('2026-09-04', TODAY)).toBe('Начало не может быть в прошлом');
    expect(rentalStartDateError(TODAY, TODAY)).toBeUndefined();
  });
});

describe('rentalPlannedEndDateError', () => {
  it('окончание строго позже начала; пустое — бессрочная, без ошибки', () => {
    expect(rentalPlannedEndDateError(undefined, '2026-09-10')).toBeUndefined();
    expect(rentalPlannedEndDateError('2026-09-10', '2026-09-10')).toBe(
      'Окончание должно быть позже начала',
    );
    expect(rentalPlannedEndDateError('2026-09-09', '2026-09-10')).toBe(
      'Окончание должно быть позже начала',
    );
    expect(rentalPlannedEndDateError('2026-09-11', '2026-09-10')).toBeUndefined();
  });

  it('без начала ошибку окончания не показываем', () => {
    expect(rentalPlannedEndDateError('2026-09-10', undefined)).toBeUndefined();
  });
});

describe('paymentDayLabel', () => {
  it('метки поля дня оплаты', () => {
    expect(paymentDayLabel(10)).toBe('10 число');
    expect(paymentDayLabel(1)).toBe('1 число');
    expect(paymentDayLabel('last')).toBe('Последний день месяца');
  });
});

describe('paymentDayPhrase', () => {
  it('фразы экрана успеха', () => {
    expect(paymentDayPhrase(10)).toBe('Каждое 10 число месяца');
    expect(paymentDayPhrase('last')).toBe('Каждый последний день месяца');
  });
});

describe('paymentDayFromPicker', () => {
  it('собирает день оплаты из выбора пикера: день и «последний день» взаимоисключимы', () => {
    expect(paymentDayFromPicker({ day: 10, last: false })).toBe(10);
    expect(paymentDayFromPicker({ day: undefined, last: true })).toBe('last');
    expect(paymentDayFromPicker({ day: 10, last: true })).toBe('last');
    expect(paymentDayFromPicker({ day: undefined, last: false })).toBeUndefined();
  });
});

describe('utilitiesLabel', () => {
  it('метки режимов коммунальных платежей', () => {
    expect(utilitiesLabel('included')).toBe('Включены в стоимость');
    expect(utilitiesLabel('meters_only')).toBe('Только счетчики');
    expect(utilitiesLabel('full_receipt')).toBe('Вся квитанция');
  });
});

describe('buildRentalCreateCommand', () => {
  it('собирает полную команду из черновика', () => {
    expect(buildRentalCreateCommand(FULL_DRAFT, TODAY)).toStrictEqual({
      amountKopecks: 5600000,
      paymentDay: 10,
      startDate: '2026-09-10',
      plannedEndDate: '2027-09-09',
      utilities: 'full_receipt',
      depositKopecks: 3000000,
      commissionKopecks: 500000,
      contactId: '0198c7a2-0000-7000-8000-000000000003',
      autoPay: false,
      reminderOffsetDays: 3,
    });
  });

  it('незаданные необязательные поля уходят null; напоминание — дефолт «За 1 день» (решение #823)', () => {
    expect(
      buildRentalCreateCommand(
        { amountKopecks: 5600000, paymentDay: 'last', startDate: TODAY },
        TODAY,
      ),
    ).toStrictEqual({
      amountKopecks: 5600000,
      paymentDay: 'last',
      startDate: TODAY,
      plannedEndDate: null,
      utilities: 'included',
      depositKopecks: null,
      commissionKopecks: null,
      contactId: null,
      autoPay: false,
      reminderOffsetDays: 1,
    });
  });

  it('выбранное напоминание протекает в команду (аренда → платёж 1:1)', () => {
    expect(
      buildRentalCreateCommand(
        { amountKopecks: 5600000, paymentDay: 10, startDate: TODAY, reminderOffsetDays: 7 },
        TODAY,
      )?.reminderOffsetDays,
    ).toBe(7);
  });

  it('недостроенный черновик команды не даёт', () => {
    expect(buildRentalCreateCommand({}, TODAY)).toBeUndefined();
    expect(buildRentalCreateCommand({ amountKopecks: 100 }, TODAY)).toBeUndefined();
    expect(buildRentalCreateCommand({ amountKopecks: 100, paymentDay: 5 }, TODAY)).toBeUndefined();
  });

  it('некорректная сумма (ноль, отрицательная) команду роняет', () => {
    expect(buildRentalCreateCommand({ amountKopecks: 0, paymentDay: 5, startDate: TODAY }, TODAY)).toBeUndefined();
    expect(buildRentalCreateCommand({ amountKopecks: -1, paymentDay: 5, startDate: TODAY }, TODAY)).toBeUndefined();
  });

  it('некорректные даты команду роняют', () => {
    expect(
      buildRentalCreateCommand({ amountKopecks: 100, paymentDay: 5, startDate: '2026-09-04' }, TODAY),
    ).toBeUndefined();
    expect(
      buildRentalCreateCommand(
        {
          amountKopecks: 100,
          paymentDay: 5,
          startDate: '2026-09-10',
          plannedEndDate: '2026-09-10',
        },
        TODAY,
      ),
    ).toBeUndefined();
  });
});
