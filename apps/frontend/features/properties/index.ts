export * from './api';

export { usePropertiesWithMeta } from './api/hooks';
export type { DeletePropertyMode } from './api/hooks';
export { getDisplayStatus, statusFilterOptions } from './lib/property-statuses';
export { canMutateProperty } from './lib/can-mutate-property';
export type { StatusFilterValue } from './lib/property-statuses';
export { propertyTypeLabels, propertyTypeOptions } from './lib/property-types';
export {
  initialPropertyCreateStep,
  isApartmentCategory,
  propertyCategoryOptions,
  propertyCreateStepReady,
  propertyHousingTypeOptions,
  PROPERTY_CREATE_TOTAL_STEPS,
} from './lib/property-create-draft';
export type {
  PropertyCreateDraft,
  PropertyCreateStep,
} from './lib/property-create-draft';
export {
  buildPropertyCreateCommand,
  type PropertyAttributesPort,
} from './lib/property-create-submit';
export { addressSuggestionRow } from './lib/address-suggestion';
export {
  PROPERTY_CREATE_RENTAL_STUB_TOAST,
  propertyCreateSuccessCopy,
} from './lib/property-create-success';
export { propertyTypeIcons } from './lib/property-type-icons';
export { usePropertyCreateDraft } from './lib/use-property-create-draft';
export { PropertyStatusBadge } from './ui/PropertyStatusBadge';
