import { beforeEach, describe, expect, it } from 'vitest';
import type { RentalEditForm } from './edit-model';
import {
  clearRentalTermsEditSessionDraft,
  consumeRentalTermsEditSessionContact,
  keepRentalTermsEditSessionAlive,
  openRentalTermsEditSession,
  readRentalTermsEditSessionDraft,
  setRentalTermsEditSessionContact,
  setRentalTermsEditSessionForm,
} from './rental-terms-edit-session';

/** Полная форма правки — каноничный фикстурный носитель. */
const FORM: RentalEditForm = {
  amountKopecks: 5600000,
  paymentDay: 10,
  autoPay: true,
  notifyEmail: true,
  reminderOffsetDays: 3,
  plannedEndDate: '2028-05-10',
  utilities: 'meters_only',
  depositKopecks: 5600000,
  commissionKopecks: null,
  comment: 'Предыдущий жилец съехал',
  contactId: null,
};

const PROPERTY = '0198c7a2-0000-7000-8000-000000000002';
const CONTACT = '0198c7a2-0000-7000-8000-0000000000aa';

/** Макротаска, в которую попадает отложенная чистка носителя. */
function tick(): Promise<void> {
  return new Promise((resolve) => {
    setTimeout(resolve, 0);
  });
}

beforeEach(() => {
  clearRentalTermsEditSessionDraft(PROPERTY);
});

describe('носитель сессии формы «Изменить условия» (#1623)', () => {
  it('гостевая запись контакта создаёт носитель с ожиданием', () => {
    expect(readRentalTermsEditSessionDraft(PROPERTY)).toStrictEqual({
      form: null,
      pendingContactId: null,
    });

    setRentalTermsEditSessionContact(PROPERTY, CONTACT);
    expect(readRentalTermsEditSessionDraft(PROPERTY).pendingContactId).toBe(CONTACT);

    // Экран-владелец потребляет ожидание один раз.
    expect(consumeRentalTermsEditSessionContact(PROPERTY)).toBe(CONTACT);
    expect(consumeRentalTermsEditSessionContact(PROPERTY)).toBeNull();
  });

  it('форма сохраняется владельцем и читается оттуда же', () => {
    setRentalTermsEditSessionForm(PROPERTY, FORM);
    expect(readRentalTermsEditSessionDraft(PROPERTY).form).toStrictEqual(FORM);
  });

  it('гостевая запись не трогает сохранённую форму', () => {
    setRentalTermsEditSessionForm(PROPERTY, FORM);
    setRentalTermsEditSessionContact(PROPERTY, CONTACT);
    const entry = readRentalTermsEditSessionDraft(PROPERTY);
    expect(entry.form).toStrictEqual(FORM);
    expect(entry.pendingContactId).toBe(CONTACT);
  });

  it('закрытие сессии без keep-alive чистит носитель отложенно', async () => {
    setRentalTermsEditSessionForm(PROPERTY, FORM);
    const close = openRentalTermsEditSession(PROPERTY);
    close();
    await tick();
    expect(readRentalTermsEditSessionDraft(PROPERTY)).toStrictEqual({
      form: null,
      pendingContactId: null,
    });
  });

  it('keep-alive переживает закрытие, повторное открытие гасит флаг', async () => {
    const close = openRentalTermsEditSession(PROPERTY); // монтирование формы
    setRentalTermsEditSessionForm(PROPERTY, FORM);
    keepRentalTermsEditSessionAlive(PROPERTY); // клик: уход в под-маршрут
    close(); // размонтирование формы — носитель переживает
    await tick();
    expect(readRentalTermsEditSessionDraft(PROPERTY).form).toStrictEqual(FORM);

    const closeAgain = openRentalTermsEditSession(PROPERTY); // возврат — флаг погашен
    closeAgain(); // уход без keep-alive — чистка
    await tick();
    expect(readRentalTermsEditSessionDraft(PROPERTY).form).toBeNull();
  });

  it('явная чистка после сохранения даёт чистый лист', () => {
    setRentalTermsEditSessionForm(PROPERTY, FORM);
    clearRentalTermsEditSessionDraft(PROPERTY);
    expect(readRentalTermsEditSessionDraft(PROPERTY).form).toBeNull();
  });
});
