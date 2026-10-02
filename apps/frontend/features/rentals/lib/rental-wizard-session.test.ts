import { beforeEach, describe, expect, it } from 'vitest';
import {
  clearRentalWizardSessionDraft,
  keepRentalWizardSessionAlive,
  openRentalWizardSession,
  readRentalWizardSessionDraft,
  setRentalWizardSessionDraft,
  validateRentalWizardDraft,
} from './rental-wizard-session';

/** Макротаска, в которую попадает отложенная чистка носителя. */
function tick(): Promise<void> {
  return new Promise((resolve) => {
    setTimeout(resolve, 0);
  });
}

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

describe('носитель сессии визарда аренды (D3)', () => {
  const APARTMENT = '0198c7a2-0000-7000-8000-000000000001';
  const GARAGE = '0198c7a2-0000-7000-8000-000000000002';

  // Носитель — модуль-синглтон: состояние течёт между кейсами, сбрасываем.
  beforeEach(() => {
    clearRentalWizardSessionDraft(APARTMENT);
    clearRentalWizardSessionDraft(GARAGE);
  });

  it('свежая сессия читается пустым черновиком, ссылка стабильна между чтениями', () => {
    expect(readRentalWizardSessionDraft(APARTMENT)).toStrictEqual({});
    expect(readRentalWizardSessionDraft(APARTMENT)).toBe(readRentalWizardSessionDraft(APARTMENT));
  });

  it('черновик пишется (значением и функцией), объекты изолированы', () => {
    setRentalWizardSessionDraft(APARTMENT, { amountKopecks: 5600000, paymentDay: 10 });
    setRentalWizardSessionDraft(APARTMENT, (prev) => ({ ...prev, startDate: '2026-10-02' }));

    expect(readRentalWizardSessionDraft(APARTMENT)).toStrictEqual({
      amountKopecks: 5600000,
      paymentDay: 10,
      startDate: '2026-10-02',
    });
    expect(readRentalWizardSessionDraft(GARAGE)).toStrictEqual({});
  });

  it('мусорное поле записи отбрасывается валидатором, валидные едут дальше', () => {
    setRentalWizardSessionDraft(APARTMENT, { amountKopecks: 5600000, paymentDay: 32 });

    expect(readRentalWizardSessionDraft(APARTMENT)).toStrictEqual({ amountKopecks: 5600000 });
  });

  it('keep-alive бережёт сессию через размонтирование (уход в под-маршрут)', async () => {
    const close = openRentalWizardSession(APARTMENT);
    setRentalWizardSessionDraft(APARTMENT, { amountKopecks: 5600000 });
    keepRentalWizardSessionAlive(APARTMENT);
    setRentalWizardSessionDraft(APARTMENT, (prev) => ({ ...prev, contactId: 'contact-1' }));
    close();
    await tick();

    expect(readRentalWizardSessionDraft(APARTMENT)).toStrictEqual({
      amountKopecks: 5600000,
      contactId: 'contact-1',
    });
  });

  it('возврат в визард гасит флаг: следующий уход чистит носитель', async () => {
    setRentalWizardSessionDraft(APARTMENT, { amountKopecks: 5600000 });
    keepRentalWizardSessionAlive(APARTMENT);
    openRentalWizardSession(APARTMENT)();

    expect(readRentalWizardSessionDraft(APARTMENT)).toStrictEqual({ amountKopecks: 5600000 });

    openRentalWizardSession(APARTMENT)();
    await tick();

    expect(readRentalWizardSessionDraft(APARTMENT)).toStrictEqual({});
  });

  it('двойное монтирование StrictMode не роняет сессию (setup → cleanup → setup)', async () => {
    setRentalWizardSessionDraft(APARTMENT, { amountKopecks: 5600000 });
    keepRentalWizardSessionAlive(APARTMENT); // клик «Выбрать контакт» перед push

    // Возврат в визард в dev-React: первый setup гасит флаг, симулированный
    // cleanup откладывает чистку, повторный setup отменяет её.
    openRentalWizardSession(APARTMENT);
    openRentalWizardSession(APARTMENT)();
    openRentalWizardSession(APARTMENT);
    await tick();

    expect(readRentalWizardSessionDraft(APARTMENT)).toStrictEqual({ amountKopecks: 5600000 });
  });

  it('уход без флага чистит носитель (браузерный «назад», финал, закрытие)', async () => {
    const close = openRentalWizardSession(APARTMENT);
    setRentalWizardSessionDraft(APARTMENT, { amountKopecks: 5600000 });
    close();
    await tick();

    expect(readRentalWizardSessionDraft(APARTMENT)).toStrictEqual({});
  });

  it('keep-alive одного объекта не задевает соседний', async () => {
    const closeApartment = openRentalWizardSession(APARTMENT);
    setRentalWizardSessionDraft(APARTMENT, { amountKopecks: 5600000 });
    setRentalWizardSessionDraft(GARAGE, { amountKopecks: 4700000 });
    keepRentalWizardSessionAlive(APARTMENT);
    closeApartment();

    const closeGarage = openRentalWizardSession(GARAGE);
    closeGarage();
    await tick();

    expect(readRentalWizardSessionDraft(APARTMENT)).toStrictEqual({ amountKopecks: 5600000 });
    expect(readRentalWizardSessionDraft(GARAGE)).toStrictEqual({});
  });

  it('clearDraft после успешного POST чистит носитель сразу', () => {
    setRentalWizardSessionDraft(APARTMENT, { amountKopecks: 5600000, paymentDay: 10 });
    clearRentalWizardSessionDraft(APARTMENT);

    expect(readRentalWizardSessionDraft(APARTMENT)).toStrictEqual({});
  });

  it('закрытие никогда не открытой сессии безопасно', async () => {
    openRentalWizardSession(GARAGE)();
    await tick();

    expect(readRentalWizardSessionDraft(GARAGE)).toStrictEqual({});
  });
});
