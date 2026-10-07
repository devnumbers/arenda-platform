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
  isApartmentCategory,
  propertyCategoryOf,
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
// Реестр живёт на слое entities (нужен PropertyAvatar), тут — обратная
// совместимость для консюмеров фичи.
export { propertyTypeIcons } from '@/entities/property';
export { PropertyStatusBadge } from './ui/PropertyStatusBadge';
