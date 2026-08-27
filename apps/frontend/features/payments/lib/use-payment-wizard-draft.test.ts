import { describe, expect, it } from 'vitest';
import {
  hasPaymentWizardDraftFields,
  paymentDraftStorageKey,
  validatePaymentWizardDraft,
  type PaymentWizardDraft,
} from './use-payment-wizard-draft';

describe('paymentDraftStorageKey', () => {
  it('разводит объекты и типы платежа по ключам', () => {
    expect(paymentDraftStorageKey('p1', 'payment')).toBe('payment-wizard-draft:p1:payment');
    expect(paymentDraftStorageKey('p1', 'autopayment')).toBe('payment-wizard-draft:p1:autopayment');
    expect(paymentDraftStorageKey('p2', 'payment')).toBe('payment-wizard-draft:p2:payment');
  });
});

describe('validatePaymentWizardDraft', () => {
  it('невалидный payload → пустой черновик', () => {
    expect(validatePaymentWizardDraft(null)).toStrictEqual({});
    expect(validatePaymentWizardDraft('строка')).toStrictEqual({});
    expect(validatePaymentWizardDraft(42)).toStrictEqual({});
    expect(validatePaymentWizardDraft({ step: 'мусор' })).toStrictEqual({});
  });

  it('сохраняет известные поля шагов', () => {
    const parsed = {
      categorySlug: 'rent',
      title: 'Арендная плата',
      recurrence: { kind: 'monthly', dayOfMonth: 1 },
      endDate: '2027-01-31',
      amountKopecks: 4500000,
      paymentForm: 'transfer',
      type: 'expense',
    };
    expect(validatePaymentWizardDraft(parsed)).toStrictEqual(parsed);
  });

  it('незнакомая регулярность роняет весь черновик', () => {
    expect(
      validatePaymentWizardDraft({ recurrence: { kind: 'hourly' }, title: 'x' }),
    ).toStrictEqual({});
  });

  it('незнакомая форма оплаты и тип роняют черновик', () => {
    expect(validatePaymentWizardDraft({ paymentForm: 'card' })).toStrictEqual({});
    expect(validatePaymentWizardDraft({ type: 'weekly' })).toStrictEqual({});
  });

  it('пустые строки и неположительные суммы отбрасываются', () => {
    expect(validatePaymentWizardDraft({ title: '' })).toStrictEqual({});
    expect(validatePaymentWizardDraft({ amountKopecks: 0 })).toStrictEqual({});
    expect(validatePaymentWizardDraft({ amountKopecks: 10.5 })).toStrictEqual({});
    expect(validatePaymentWizardDraft({ amountKopecks: -5 })).toStrictEqual({});
  });

  it('пустая запись — валидный пустой черновик', () => {
    expect(validatePaymentWizardDraft({})).toStrictEqual({});
  });
});

describe('hasPaymentWizardDraftFields', () => {
  it('пустой черновик — не черновик', () => {
    const empty: PaymentWizardDraft = {};
    expect(hasPaymentWizardDraftFields(empty)).toBe(false);
  });

  it('любое заполненное поле делает черновик наличным', () => {
    expect(hasPaymentWizardDraftFields({ categorySlug: 'rent' })).toBe(true);
    expect(hasPaymentWizardDraftFields({ endDate: '2027-01-31' })).toBe(true);
  });
});
