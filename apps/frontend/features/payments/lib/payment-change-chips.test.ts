import { describe, expect, it } from 'vitest';
import type { PaymentChangeEntry, PaymentFieldChange } from '@/entities/payment';
import { paymentChangeChips } from './payment-change-chips';

/** Строка журнала фикстурой: правка с одним полем — сценарные строки
 * задаются полем дифа и действием (макеты 3214-73216, #1195). */
function entry(
  changes: ReadonlyArray<PaymentFieldChange>,
  overrides: Partial<PaymentChangeEntry> = {},
): PaymentChangeEntry {
  return {
    id: 'ch-1',
    action: 'updated',
    changes,
    createdAt: '2026-09-01T10:00:00Z',
    ...overrides,
  };
}

describe('paymentChangeChips — правка полей (макет 3214-73216)', () => {
  it('сумма: новое значение каноническим денежным форматтером', () => {
    expect(
      paymentChangeChips(
        entry([{ field: 'amount_kopecks', old: 250_000, new: 180_000 }]),
      ),
    ).toEqual(['Сумма изменена: 1\u00A0800 ₽']);
  });

  it('название: новое значение в кавычках-ёлочках', () => {
    expect(
      paymentChangeChips(entry([{ field: 'title', old: 'Юрист', new: 'Документы' }])),
    ).toEqual(['Название изменено: «Документы»']);
  });

  it('категория: лейбл-снапшот значения в кавычках, не текущее имя категории', () => {
    expect(
      paymentChangeChips(
        entry([
          {
            field: 'category_slug',
            old: { slug: 'legal', label: 'Юридические услуги' },
            new: { slug: 'notary', label: 'Нотариальный услуги' },
          },
        ]),
      ),
    ).toEqual(['Категория изменена: «Нотариальный услуги»']);
  });

  it('регулярность: канонический recurrenceLabel нового значения', () => {
    expect(
      paymentChangeChips(
        entry([
          {
            field: 'recurrence',
            old: { kind: 'monthly', daysOfMonth: [1], lastDay: false },
            new: { kind: 'monthly', daysOfMonth: [1, 10, 15], lastDay: false },
          },
        ]),
      ),
    ).toEqual(['Регулярность изменена: Каждый месяц 1, 10 и 15 числа']);
  });

  it('окончание добавлено: старого срока не было', () => {
    expect(
      paymentChangeChips(entry([{ field: 'end_date', old: null, new: '2027-05-01' }])),
    ).toEqual(['Окончание платежа добавлено: 1 мая, 2027']);
  });

  it('окончание изменено на дату', () => {
    expect(
      paymentChangeChips(
        entry([{ field: 'end_date', old: '2027-05-01', new: '2028-12-01' }]),
      ),
    ).toEqual(['Окончание платежа изменено: 1 декабря, 2028']);
  });

  it('окончание изменено на бессрочное: канон «Бессрочно»', () => {
    expect(
      paymentChangeChips(entry([{ field: 'end_date', old: '2028-12-01', new: null }])),
    ).toEqual(['Окончание платежа изменено: Бессрочно']);
  });

  it('направление: фраза без значения, «доход → расход»', () => {
    expect(
      paymentChangeChips(entry([{ field: 'type', old: 'income', new: 'expense' }])),
    ).toEqual(['Доход изменен на расход']);
  });

  it('направление: «расход → доход»', () => {
    expect(
      paymentChangeChips(entry([{ field: 'type', old: 'expense', new: 'income' }])),
    ).toEqual(['Расход изменен на доход']);
  });

  it('автоплатёж включен и выключен', () => {
    expect(
      paymentChangeChips(entry([{ field: 'auto_pay', old: false, new: true }])),
    ).toEqual(['Автоплатеж включен']);
    expect(
      paymentChangeChips(entry([{ field: 'auto_pay', old: true, new: false }])),
    ).toEqual(['Автоплатеж выключен']);
  });

  it('напоминание добавлено и изменено: метки пикера («За 3 дня»)', () => {
    expect(
      paymentChangeChips(entry([{ field: 'reminder_offset_days', old: null, new: 1 }])),
    ).toEqual(['Напоминание добавлено: За 1 день']);
    expect(
      paymentChangeChips(entry([{ field: 'reminder_offset_days', old: 1, new: 7 }])),
    ).toEqual(['Напоминание изменено: За 7 дней']);
  });

  it('напоминание снято: канон пикера «Не напоминать»', () => {
    expect(
      paymentChangeChips(entry([{ field: 'reminder_offset_days', old: 3, new: null }])),
    ).toEqual(['Напоминание изменено: Не напоминать']);
  });

  it('одна правка — несколько полей: чипы в словарном порядке дифа', () => {
    expect(
      paymentChangeChips(
        entry([
          { field: 'amount_kopecks', old: 250_000, new: 250_000 },
          { field: 'type', old: 'income', new: 'expense' },
          { field: 'title', old: 'Охрана', new: 'Юрист' },
          {
            field: 'category_slug',
            old: { slug: 'guard', label: 'Охрана' },
            new: { slug: 'legal', label: 'Юридические услуги' },
          },
        ]),
      ),
    ).toEqual([
      'Сумма изменена: 2\u00A0500 ₽',
      'Доход изменен на расход',
      'Название изменено: «Юрист»',
      'Категория изменена: «Юридические услуги»',
    ]);
  });
});

describe('paymentChangeChips — пауза и возобновление (ADR 0065 §4)', () => {
  it('пауза — строка с пустым дифом, текст тостов мутаций (#452)', () => {
    expect(paymentChangeChips(entry([], { action: 'paused' }))).toEqual([
      'Платеж поставлен на паузу',
    ]);
  });

  it('возобновление — строка с пустым дифом', () => {
    expect(paymentChangeChips(entry([], { action: 'resumed' }))).toEqual([
      'Платеж возобновлен',
    ]);
  });

  it('обновление без дифа не даёт чипов (пустая строка не рисуется)', () => {
    expect(paymentChangeChips(entry([]))).toEqual([]);
  });
});
