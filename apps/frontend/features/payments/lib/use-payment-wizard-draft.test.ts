import { describe, expect, it } from 'vitest';
import {
  hasPaymentWizardDraftFields,
  latestPaymentDraftType,
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
      recurrence: { kind: 'monthly', daysOfMonth: [1], lastDay: false },
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

  it('напоминание сохраняется, чужой оффал роняет черновик (карта #822)', () => {
    expect(validatePaymentWizardDraft({ title: 'x', reminderOffsetDays: 3 })).toStrictEqual({
      title: 'x',
      reminderOffsetDays: 3,
    });
    expect(validatePaymentWizardDraft({ reminderOffsetDays: 2 })).toStrictEqual({});
    expect(validatePaymentWizardDraft({ reminderOffsetDays: 'завтра' })).toStrictEqual({});
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

  it('служебный updatedAt проходит при валидности и отбрасывается при мусоре', () => {
    expect(validatePaymentWizardDraft({ title: 'x', updatedAt: 1756400000000 })).toStrictEqual({
      title: 'x',
      updatedAt: 1756400000000,
    });
    // Мусорный таймстамп не роняет черновик — только сам отбрасывается.
    expect(validatePaymentWizardDraft({ title: 'x', updatedAt: 0 })).toStrictEqual({ title: 'x' });
    expect(validatePaymentWizardDraft({ title: 'x', updatedAt: -5 })).toStrictEqual({ title: 'x' });
    expect(validatePaymentWizardDraft({ title: 'x', updatedAt: 10.5 })).toStrictEqual({ title: 'x' });
    expect(validatePaymentWizardDraft({ title: 'x', updatedAt: 'вчера' })).toStrictEqual({ title: 'x' });
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
    expect(hasPaymentWizardDraftFields({ reminderOffsetDays: 7 })).toBe(true);
  });

  it('служебный updatedAt сам по себе черновиком не считается', () => {
    expect(hasPaymentWizardDraftFields({ updatedAt: 1756400000000 })).toBe(false);
  });
});

describe('latestPaymentDraftType', () => {
  const state = (hasDraft: boolean, updatedAt?: number) => ({
    hasDraft,
    draft: updatedAt === undefined ? {} : { updatedAt },
  });

  it('черновиков нет — undefined', () => {
    expect(latestPaymentDraftType(state(false), state(false))).toBeUndefined();
  });

  it('один черновик — он и последний', () => {
    expect(latestPaymentDraftType(state(true, 100), state(false))).toBe('payment');
    expect(latestPaymentDraftType(state(false), state(true, 100))).toBe('autopayment');
  });

  it('два черновика — свежий по updatedAt', () => {
    expect(latestPaymentDraftType(state(true, 100), state(true, 200))).toBe('autopayment');
    expect(latestPaymentDraftType(state(true, 300), state(true, 200))).toBe('payment');
  });

  it('равная свежесть (унаследованные без таймстампов) — платёж', () => {
    expect(latestPaymentDraftType(state(true), state(true))).toBe('payment');
    expect(latestPaymentDraftType(state(true, 100), state(true, 100))).toBe('payment');
  });
});
