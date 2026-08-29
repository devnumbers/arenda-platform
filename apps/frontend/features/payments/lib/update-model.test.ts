import { describe, expect, it } from 'vitest';
import type { Payment } from '@/entities/payment';
import {
  buildPaymentUpdateCommand,
  editFormReady,
  type PaymentEditForm,
} from './update-model';

/** Правило-база: все поля заполнены, категория из дефолтного каталога. */
function basePayment(overrides: Partial<Payment> = {}): Payment {
  return {
    id: '019abcde-0000-7000-8000-000000000001',
    propertyId: '019abcde-0000-7000-8000-000000000002',
    type: 'expense',
    title: 'Страхование',
    amountKopecks: 320_000,
    recurrence: { kind: 'monthly', daysOfMonth: [15], lastDay: false },
    since: '2026-08-01',
    endDate: undefined,
    autoPay: false,
    paymentForm: 'cash',
    category: { source: 'default', slug: 'insurance', label: 'Страхование' },
    isFavorite: false,
    pauses: [],
    createdAt: '2026-08-01T10:00:00Z',
    updatedAt: '2026-08-01T10:00:00Z',
    ...overrides,
  };
}

/** Форма-база: значения совпадают с правилом-базой. */
function baseForm(overrides: Partial<PaymentEditForm> = {}): PaymentEditForm {
  return {
    type: 'expense',
    title: 'Страхование',
    amountKopecks: 320_000,
    categorySlug: undefined,
    paymentForm: 'cash',
    recurrence: { kind: 'monthly', daysOfMonth: [15], lastDay: false },
    endDate: undefined,
    ...overrides,
  };
}

describe('buildPaymentUpdateCommand', () => {
  it('без изменений возвращает undefined — PATCH не отправляется', () => {
    expect(buildPaymentUpdateCommand(basePayment(), baseForm())).toBeUndefined();
  });

  it('изменённая сумма попадает в команду, остальное опущено', () => {
    const command = buildPaymentUpdateCommand(basePayment(), baseForm({ amountKopecks: 400_000 }));
    expect(command).toEqual({ amountKopecks: 400_000 });
  });

  it('изменённые тип, форма оплаты и название идут одним PATCH', () => {
    const command = buildPaymentUpdateCommand(
      basePayment(),
      baseForm({ type: 'income', paymentForm: 'transfer', title: '  Страхование квартиры  ' }),
    );
    expect(command).toEqual({
      type: 'income',
      paymentForm: 'transfer',
      title: 'Страхование квартиры',
    });
  });

  it('изменённая регулярность сравнивается по содержимому', () => {
    const command = buildPaymentUpdateCommand(
      basePayment(),
      baseForm({ recurrence: { kind: 'monthly', daysOfMonth: [20], lastDay: false } }),
    );
    expect(command).toEqual({ recurrence: { kind: 'monthly', daysOfMonth: [20], lastDay: false } });
  });

  it('порядок дней недели не ломает сравнение еженедельной регулярности', () => {
    const payment = basePayment({
      recurrence: { kind: 'weekly', weekdays: [1, 3] },
    });
    expect(
      buildPaymentUpdateCommand(payment, baseForm({
        recurrence: { kind: 'weekly', weekdays: [3, 1] },
      })),
    ).toBeUndefined();
    expect(
      buildPaymentUpdateCommand(payment, baseForm({
        recurrence: { kind: 'weekly', weekdays: [1] },
      })),
    ).toEqual({ recurrence: { kind: 'weekly', weekdays: [1] } });
  });

  describe('endDate трисостоянен', () => {
    it('пусто и было пусто — опущено', () => {
      expect(buildPaymentUpdateCommand(basePayment(), baseForm())).toBeUndefined();
    });

    it('дата назначена — в команде дата', () => {
      const command = buildPaymentUpdateCommand(
        basePayment(),
        baseForm({ endDate: '2027-08-15' }),
      );
      expect(command).toEqual({ endDate: '2027-08-15' });
    });

    it('была дата, стало пусто — null открывает срок', () => {
      const payment = basePayment({ endDate: '2027-08-15' });
      const command = buildPaymentUpdateCommand(payment, baseForm());
      expect(command).toEqual({ endDate: null });
    });

    it('та же дата — опущено', () => {
      const payment = basePayment({ endDate: '2027-08-15' });
      expect(
        buildPaymentUpdateCommand(payment, baseForm({ endDate: '2027-08-15' })),
      ).toBeUndefined();
    });
  });

  describe('категория', () => {
    it('выбор нового слага дефолтного каталога попадает в команду', () => {
      const command = buildPaymentUpdateCommand(
        basePayment(),
        baseForm({ categorySlug: 'internet' }),
      );
      expect(command).toEqual({ categorySlug: 'internet' });
    });

    it('тот же слаг — опущен', () => {
      expect(
        buildPaymentUpdateCommand(basePayment(), baseForm({ categorySlug: 'insurance' })),
      ).toBeUndefined();
    });

    it('пользовательская категория без выбора — категория не уходит в PATCH', () => {
      const payment = basePayment({
        category: { source: 'custom', id: '019abcde-0000-7000-8000-000000000003', label: 'Моя' },
      });
      expect(buildPaymentUpdateCommand(payment, baseForm())).toBeUndefined();
    });
  });

  it('пустое название откатывается к лейблу категории резолвером', () => {
    const resolveTitle = (slug: string) => (slug === 'insurance' ? 'Страхование' : undefined);
    const command = buildPaymentUpdateCommand(
      basePayment({ title: 'Страхование' }),
      baseForm({ title: '   ' }),
      { resolveTitle },
    );
    expect(command).toBeUndefined();

    const renamed = buildPaymentUpdateCommand(
      basePayment({ title: 'Кредит' }),
      baseForm({ title: '' }),
      { resolveTitle },
    );
    expect(renamed).toEqual({ title: 'Страхование' });
  });

  it('пустое название с новой категорией берёт лейбл новой категории', () => {
    const command = buildPaymentUpdateCommand(
      basePayment({ title: 'Кредит' }),
      baseForm({ title: '', categorySlug: 'internet' }),
      { resolveTitle: (slug) => (slug === 'internet' ? 'Интернет' : undefined) },
    );
    expect(command).toEqual({
      title: 'Интернет',
      categorySlug: 'internet',
    });
  });

  it('пользовательская категория без выбора: пустое название не резолвится', () => {
    const payment = basePayment({
      title: 'Моя категория',
      category: { source: 'custom', id: '019abcde-0000-7000-8000-000000000003', label: 'Моя' },
    });
    expect(buildPaymentUpdateCommand(payment, baseForm({ title: '' }))).toBeUndefined();
  });
});

describe('editFormReady', () => {
  it('заполненная форма готова', () => {
    expect(editFormReady(baseForm(), 'insurance')).toBe(true);
  });

  it('не готова без суммы или с нулевой суммой', () => {
    expect(editFormReady(baseForm({ amountKopecks: undefined }), 'insurance')).toBe(false);
    expect(editFormReady(baseForm({ amountKopecks: 0 }), 'insurance')).toBe(false);
  });

  it('не готова, когда название пусто и категории-фолбэка нет', () => {
    expect(
      editFormReady(baseForm({ title: '' }), undefined, () => 'Лейбл'),
    ).toBe(false);
    expect(
      editFormReady(baseForm({ title: '' }), 'insurance', () => 'Лейбл из каталога'),
    ).toBe(true);
  });

  it('неготовая регулярность закрывает сохранение', () => {
    expect(
      editFormReady(baseForm({ recurrence: { kind: 'weekly', weekdays: [] } }), 'insurance'),
    ).toBe(false);
  });
});
