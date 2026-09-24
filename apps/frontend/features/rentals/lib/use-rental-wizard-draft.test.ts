import { describe, expect, it } from 'vitest';
import { validateRentalWizardDraft } from './use-rental-wizard-draft';

describe('validateRentalWizardDraft', () => {
  it('валидный черновик проходит как есть', () => {
    expect(
      validateRentalWizardDraft({
        amountKopecks: 5600000,
        paymentDay: 10,
        autoPay: true,
        startDate: '2026-09-10',
        plannedEndDate: '2027-09-09',
        utilities: 'full_receipt',
        depositKopecks: 0,
        commissionKopecks: 500000,
        contactId: '0198c7a2-0000-7000-8000-000000000003',
        reminderOffsetDays: 7,
      }),
    ).toStrictEqual({
      amountKopecks: 5600000,
      paymentDay: 10,
      autoPay: true,
      startDate: '2026-09-10',
      plannedEndDate: '2027-09-09',
      utilities: 'full_receipt',
      depositKopecks: 0,
      commissionKopecks: 500000,
      contactId: '0198c7a2-0000-7000-8000-000000000003',
      reminderOffsetDays: 7,
    });
  });

  it('«последний день месяца» — валидный день оплаты', () => {
    expect(validateRentalWizardDraft({ paymentDay: 'last' })).toStrictEqual({
      paymentDay: 'last',
    });
  });

  it('мусорные значения полей отбрасываются по одному', () => {
    expect(
      validateRentalWizardDraft({
        amountKopecks: 0,
        paymentDay: 32,
        autoPay: 'да',
        startDate: '10.09.2026',
        plannedEndDate: '',
        utilities: 'все',
        depositKopecks: -5,
        commissionKopecks: 1.5,
        contactId: '',
      }),
    ).toStrictEqual({});
  });

  it('день оплаты вне 1–31 и не-числа не проходят', () => {
    expect(validateRentalWizardDraft({ paymentDay: 'десятое' })).toStrictEqual({});
    expect(validateRentalWizardDraft({ paymentDay: 0 })).toStrictEqual({});
    expect(validateRentalWizardDraft({ paymentDay: 1.5 })).toStrictEqual({});
  });

  it('напоминание — только контрактные оффсеты 1/3/7, прочие роняются', () => {
    expect(validateRentalWizardDraft({ reminderOffsetDays: 3 })).toStrictEqual({
      reminderOffsetDays: 3,
    });
    expect(validateRentalWizardDraft({ reminderOffsetDays: 5 })).toStrictEqual({});
    expect(validateRentalWizardDraft({ reminderOffsetDays: 0 })).toStrictEqual({});
    expect(validateRentalWizardDraft({ reminderOffsetDays: 'три' })).toStrictEqual({});
  });

  it('не-объект и массив роняют весь черновик', () => {
    expect(validateRentalWizardDraft(null)).toStrictEqual({});
    expect(validateRentalWizardDraft('draft')).toStrictEqual({});
    expect(validateRentalWizardDraft([])).toStrictEqual({});
  });
});
