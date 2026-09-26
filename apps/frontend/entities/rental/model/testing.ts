import type { Rental } from './types';

/**
 * Общий тестовый билдер Rental вместо копии в каждом тестовом файле
 * (прецедент shared/api/sse-test-fakes). Модуль только для тестов: в
 * продукте не импортируется — наружу отдаётся тест-онли реэкспортом через
 * index.ts слайца. Канон — активная срочная аренда без арендатора и залога
 * (6 из 24 месяцев, 150 дней до платежа); сценарные значения из макетов
 * задаются overrides или тонкой локальной обёрткой, перечисляющей только
 * отличия от канона — новое обязательное поле типа правится здесь, а не
 * по тест-файлам (#849).
 */
export function makeRental(overrides: Partial<Rental> = {}): Rental {
  return {
    id: 'rental-1',
    propertyId: 'property-1',
    status: 'active',
    startDate: '2026-04-01',
    plannedEndDate: '2028-04-01',
    completedDate: null,
    utilities: 'included',
    depositKopecks: null,
    commissionKopecks: null,
    depositReturnKopecks: null,
    depositReturnComment: null,
    tenant: null,
    comment: '',
    rentPayment: {
      paymentId: 'payment-1',
      amountKopecks: 5_600_000,
      paymentDay: 1,
      autoPay: false,
      nextPayment: {
        operationId: 'operation-1',
        date: '2027-02-08',
        amountKopecks: 5_600_000,
        daysUntil: 150,
      },
    },
    progress: { paidMonths: 6, totalMonths: 24, monthsRemaining: 23, overdueMonths: null },
    today: '2026-09-11',
    createdAt: '2026-04-01T00:00:00Z',
    ...overrides,
  };
}
