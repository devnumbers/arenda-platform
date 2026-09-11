export * from './api';

export { usePropertiesWithMeta, usePropertiesLandingHref } from './api/hooks';
export type { DeletePropertyMode, PropertiesListResult } from './api/hooks';
export { canMutateProperty } from './lib/can-mutate-property';
export { propertyTypeLabels, propertyTypeOptions } from './lib/property-types';
export {
  archivedPropertyBadge,
  hasPropertyAttentionDot,
  propertyBadges,
  type PropertyBadge,
  type PropertyBadgeKey,
  type PropertyBadgeTone,
} from './lib/property-badges';
export { resolvePropertiesLandingHref } from './lib/property-landing';
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
export {
  buildPropertyEditCommand,
  initialPropertyEditDraft,
  propertyEditDirty,
  type PropertyEditDraft,
} from './lib/property-edit';
export { addressSuggestionRow } from './lib/address-suggestion';
export {
  PROPERTY_CREATE_RENTAL_STUB_TOAST,
  propertyCreateSuccessCopy,
} from './lib/property-create-success';
export { propertyTypeIcons } from './lib/property-type-icons';
export { usePropertyCreateDraft } from './lib/use-property-create-draft';
export { PropertyStatusBadge } from './ui/PropertyStatusBadge';
