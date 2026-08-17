/**
 * Модель аренды. Живёт в shared, потому что на Lease ссылаются несколько
 * entity-слайсов (property, tenant-contact, lease), а cross-slice импорты в
 * слое entities запрещены (enforced by boundaries/dependencies в
 * eslint.config.mjs). entities/lease/model/types.ts переприимпортован как
 * публичный API слайса.
 */
export type LeaseStatus =
  | 'awaiting_start'
  | 'active'
  | 'requires_action'
  | 'completed'
  | 'archived';

export type TenantContact = {
  readonly id: string;
  readonly name: string;
  readonly surname?: string | null;
  readonly patronymic?: string | null;
  readonly phone?: string | null;
  readonly email?: string | null;
};

export type Lease = {
  readonly id: string;
  readonly propertyId: string | null;
  readonly tenantContactId?: string;
  readonly tenantName: string;
  readonly tenantContact?: TenantContact | null;
  readonly rentKopecks: number;
  readonly startDate: string;
  readonly endDate?: string;
  readonly paymentDay: number;
  readonly currentPeriodOverdue: boolean;
  readonly hasOverdue: boolean;
  readonly overdueSince?: string;
  readonly nextPaymentDate?: string;
  readonly status: LeaseStatus;
};
