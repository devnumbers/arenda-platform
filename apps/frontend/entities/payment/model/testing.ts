import type { Payment } from './types';

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
