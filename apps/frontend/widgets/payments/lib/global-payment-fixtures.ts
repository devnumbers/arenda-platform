import type { GlobalPayment } from '@/entities/payment';

/** Тестовая фабрика строки глобального фида платежей (#575): канонический
 * базовый экземпляр с переопределениями — общая для модельных тестов
 * виджета (код-ревью #580: фабрика item() не должна копиться по файлам). */
export function makeGlobalPayment(
  overrides: Partial<GlobalPayment> = {},
): GlobalPayment {
  return {
    id: 'payment-1',
    propertyId: 'property-1',
    propertyName: 'Моя квартира',
    title: 'Страхование',
    amountKopecks: 3_200_000,
    type: 'expense',
    category: { source: 'default', slug: 'insurance', label: 'Страхование' },
    autoPay: false,
    isFavorite: false,
    favoriteOrder: null,
    today: '2026-09-08',
    nearestDate: '2026-09-10',
    overdueOperationCount: 0,
    overdueDays: null,
    oldestOverdueOperationId: null,
    ...overrides,
  };
}
