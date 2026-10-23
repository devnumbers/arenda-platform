import type {
  Payment,
  PaymentDueItem,
  PaymentOperation,
} from './types';

/**
 * Общий тестовый билдер Payment вместо копии в каждом тестовом файле
 * (прецедент shared/api/sse-test-fakes). Модуль только для тестов: в
 * продукте не импортируется — наружу отдаётся тест-онли реэкспортом через
 * index.ts слайца. Канон — дефолтное правило «Арендная плата» (monthly,
 * 1-е число); сценарные значения из макетов задаются overrides или тонкой
 * локальной обёрткой, перечисляющей только отличия от канона — новое
 * обязательное поле типа правится здесь, а не по тест-файлам (#849).
 */
export function makePayment(overrides: Partial<Payment> = {}): Payment {
  return {
    id: 'p1',
    propertyId: 'prop1',
    type: 'expense',
    title: 'Арендная плата',
    amountKopecks: 4_500_000,
    recurrence: { kind: 'monthly', daysOfMonth: [1], lastDay: false },
    since: '2026-01-01',
    autoPay: false,
    notifyAutoPaid: false,
    // Контракт #1553 — бэкфилл «включена»: хранёные правила несут true.
    notifyEmail: true,
    category: { source: 'default', slug: 'rent', label: 'Арендная плата' },
    isFavorite: false,
    isCompleted: false,
    isRentalManaged: false,
    isRentalCompleted: false,
    // Канон правила monthly-1: серверная «Следующая дата оплаты» — 1-е
    // следующего месяца; сценарные значения (null паузы/завершённого,
    // предоплата) задаются overrides.
    nearestDate: '2026-09-01',
    pauses: [],
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
    ...overrides,
  };
}

/**
 * Общий тестовый билдер строки «На оплату» (#1346): канон — today-строка
 * («Сегодня», daysLeft 0) правила без автоплатежа с напоминанием N=3;
 * сценарные бакеты и суммы задаются overrides.
 */
export function makePaymentDueItem(
  overrides: Partial<PaymentDueItem> = {},
): PaymentDueItem {
  return {
    paymentId: 'p1',
    operationId: 'o1',
    propertyId: 'prop1',
    propertyName: 'Моя квартира',
    title: 'Арендная плата',
    amountKopecks: 4_500_000,
    type: 'expense',
    category: { source: 'default', slug: 'rent', label: 'Арендная плата' },
    today: '2026-10-13',
    dueDate: '2026-10-13',
    daysLeft: 0,
    reminderOffsetDays: 3,
    ...overrides,
  };
}

/**
 * Общий тестовый билдер операции правила: канон — плановое вхождение
 * «Арендной платы» (monthly, 1-е число, канон makePayment); сценарные
 * статусы, факты и хвосты удалённого правила (paymentId null + origin,
 * #1371) задаются overrides.
 */
export function makePaymentOperation(
  overrides: Partial<PaymentOperation> = {},
): PaymentOperation {
  return {
    id: 'o1',
    propertyId: 'prop1',
    paymentId: 'p1',
    origin: 'payment',
    date: '2026-09-01',
    status: 'planned',
    type: 'expense',
    title: 'Арендная плата',
    amountKopecks: 4_500_000,
    categoryLabel: 'Арендная плата',
    categorySlug: 'rent',
    updatedAt: '2026-01-01T00:00:00Z',
    ...overrides,
  };
}
