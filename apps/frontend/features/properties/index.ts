export * from './api';

export { usePropertiesWithMeta, usePropertiesNavItem } from './api/hooks';
export type { PropertiesListResult, SuspendedSharedProperty } from './api/hooks';
export { propertyAutonamePhrase, propertyTypeLabels, propertyTypeOptions } from './lib/property-types';
export {
  archivedPropertyBadge,
  hasPropertyAttentionDot,
  propertyBadges,
  type PropertyBadge,
  type PropertyBadgeKey,
  type PropertyBadgeTone,
} from './lib/property-badges';
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
export { propertyCreateSuccessCopy } from './lib/property-create-success';
export { propertyTypeIcons } from './lib/property-type-icons';
export { usePropertyCreateDraft } from './lib/use-property-create-draft';
export { PropertyStatusBadge } from './ui/PropertyStatusBadge';
