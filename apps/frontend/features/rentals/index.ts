export {
  fetchRentals,
  rentalsQueryOptions,
} from './api/queries';
export {
  useCompleteRental,
  useCreateRental,
  useDeleteRental,
  useRentalSummary,
  useRentals,
  useUpdateRental,
} from './api/hooks';
export { completedRentalOperationsScope } from './lib/completed-operations';
export {
  buildRentalCreateCommand,
  draftAfterPaymentDayChange,
  draftAfterStartChange,
  paymentDayFromPicker,
  paymentDayLabel,
  plannedEndDateMinDate,
  rentalPlannedEndDateError,
  rentalStartDateError,
  utilitiesLabel,
  UTILITIES_OPTIONS,
  wizardStepReady,
  WIZARD_TOTAL_STEPS,
} from './lib/wizard-model';
export type { RentalWizardDraft, RentalWizardStep } from './lib/wizard-model';
export { rentalActionState } from './lib/rental-actions';
export type { RentalActionState } from './lib/rental-actions';
export {
  buildRentalUpdateCommand,
  formAfterPaymentDayChange,
  rentalEditFormFromRental,
  rentalPlannedEndDateEditError,
  RENTAL_COMMENT_MAX,
} from './lib/edit-model';
export type { RentalEditForm } from './lib/edit-model';
export {
  buildRentalCompleteCommand,
  completePlannedEndDate,
  rentalDurationLine,
} from './lib/complete-model';
export type { RentalCompleteDraft } from './lib/complete-model';
export {
  setRentalWizardSessionDraft,
  useRentalWizardSession,
} from './lib/rental-wizard-session';
export {
  consumeRentalCompleteResume,
  fulfilRentalCompleteResume,
  requestRentalCompleteResume,
} from './lib/rental-complete-resume-session';
export type { RentalCompleteResumeAction } from './lib/rental-complete-resume-session';
export {
  rentalCompletedTitle,
  rentalExtendSuccessCopy,
  rentalSuccessCopy,
} from './lib/success-copy';
export { rentalExtendMinDate } from './lib/extend-model';
export {
  currentRentalOf,
  hasProgressCard,
  rentalCommentText,
  rentalElapsedLine,
  rentalNextPaymentLine,
  rentalOverdueLine,
  rentalPaidTitle,
  rentalProgressPercent,
  rentalTeaserRows,
  rentalTermsRows,
  rentalTenantTitle,
  rentAmountPerMonth,
} from './lib/rental-view';
export type { RentalTermsRow } from './lib/rental-view';
// Строка остатка — общая для детализации, блока объекта и бейджа списка
// (#1387): канон живёт в shared, бейджу features/properties он нужен так же,
// как строкам аренды (прецедент months-word #851).
export { rentalRemainingLine } from '@/shared/lib/rental-remaining-line';
export {
  completedRentalMonths,
  completedRentalsOf,
  pastRentalCardTitle,
  pastRentalRows,
  pastRentalTitle,
} from './lib/past-model';
