export { mapRental, mapRentalSummary } from './model/mappers';
export type {
  Rental,
  RentalCompleteCommand,
  RentalCreateCommand,
  RentalNextPayment,
  RentalPaymentDay,
  RentalPaymentView,
  RentalProgress,
  RentalStatus,
  RentalSummary,
  RentalTenant,
  RentalUpdateCommand,
  RentalUtilities,
} from './model/types';
// Тест-онли: fixture-билдер для юнит-тестов, в продукте не импортируется
// (прецедент shared/api/sse-test-fakes).
export { makeRental } from './model/testing';
