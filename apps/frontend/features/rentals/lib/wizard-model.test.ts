import { describe, expect, it } from 'vitest';
import {
  buildRentalCreateCommand,
  draftAfterStartChange,
  paymentDayFromPicker,
  paymentDayLabel,
  paymentDayPhrase,
  rentalPlannedEndDateError,
  rentalStartDateError,
  utilitiesLabel,
  wizardStepReady,
} from './wizard-model';
import type { RentalWizardDraft } from './use-rental-wizard-draft';

const TODAY = '2026-09-05';

const FULL_DRAFT: RentalWizardDraft = {
  amountKopecks: 5600000,
  paymentDay: 10,
  autoPay: false,
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

  it('прочие поля переносятся', () => {
    expect(
      draftAfterStartChange(
        { amountKopecks: 100, paymentDay: 5, plannedEndDate: '2026-09-10' },
        '2026-09-11',
      ),
    ).toStrictEqual({ amountKopecks: 100, paymentDay: 5, startDate: '2026-09-11' });
  });

  it('без начала окончание не трогается', () => {
    expect(draftAfterStartChange({ plannedEndDate: '2026-09-10' }, undefined)).toStrictEqual({
      plannedEndDate: '2026-09-10',
    });
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
    });
  });

  it('незаданные необязательные поля уходят null, автоплатёж — дефолт false', () => {
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
    });
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
